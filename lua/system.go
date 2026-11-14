package ecslua

import (
	"fmt"
	"io/fs"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
)

// System runs one Lua script as a [system.Runner]. Create it with
// [Runtime.NewSystem]. It also implements [system.Initializer]: the script is
// loaded and checked in Init, so mistakes surface before the first tick.
//
// If Init fails and Options.OnError returns instead of panicking, the system
// stays disabled and Run does nothing, until the script file changes (see
// Options.WatchInterval) or [System.Reload] succeeds.
type System struct {
	rt  *Runtime
	src source

	loaded // the running version of the script; zero while disabled

	// Hot reload bookkeeping (doc/guide/runtime.md#hot-reload).
	loadedStat fileVersion   // version of the file that loaded (or last tried to)
	failedStat fileVersion   // version that failed to load and was reported
	elapsed    time.Duration // sum of ctx.Time.Delta, the watcher's clock
	nextCheck  time.Duration // elapsed value at which the file is stat'ed next
}

// loaded is one successfully loaded version of a script. A reload builds a
// candidate and replaces the running one only when it is complete.
type loaded struct {
	obj    *objectBinding
	query  ecs.Query
	update *lua.LFunction

	// Reused for the whole loop (see objectRef.slots): the object userdata
	// and its ref are re-pointed at each entity instead of allocated.
	ref   *objectRef
	objUD *lua.LUserData
}

// fileVersion identifies a state of the script file. Modtime and size
// together catch edits that keep the size, and edits within one timestamp tick.
type fileVersion struct {
	mtime time.Time
	size  int64
}

var (
	_ system.Runner      = (*System)(nil)
	_ system.Initializer = (*System)(nil)
)

// NewSystem returns a system that runs the script at path every tick for each
// entity of the object the script names (doc/guide/scripts.md).
func (rt *Runtime) NewSystem(path string) *System {
	return &System{rt: rt, src: source{name: path}}
}

// NewSystemFS is [Runtime.NewSystem] for a script inside an fs.FS, such as an
// embed.FS (doc/guide/runtime.md#scripts-from-an-fsfs). name must be a valid
// fs.FS path (slash-separated, unrooted); it is also the script name in
// errors and in OnError.
//
// Panics with "lua: invalid script name <name>" if name is not valid for fsys,
// and with "lua: nil fs" if fsys is nil.
func (rt *Runtime) NewSystemFS(fsys fs.FS, name string) *System {
	if fsys == nil {
		panic("lua: nil fs")
	}
	if !fs.ValidPath(name) {
		panic("lua: invalid script name " + name)
	}
	return &System{rt: rt, src: source{fsys: fsys, name: name}}
}

// Init loads the script, resolves its object and builds the query. It runs
// once, before the first stage of the first tick.
//
// Panics with "lua: system used with a different world" if ctx.World is not
// the Runtime's World.
func (s *System) Init(ctx *system.Context) {
	rt := s.rt
	if ctx.World != rt.world {
		panic("lua: system used with a different world")
	}
	rt.cmds = ctx.Commands
	defer func() { rt.cmds = nil }()

	if err := s.reload(); err != nil {
		rt.fail(s.src.name, err)
	}
}

// Reload loads the script file now and, if it is valid, replaces the running
// version with it. On error the previous version keeps running and the error
// is returned (OnError is not called). A script reloaded this way starts with a
// fresh environment and its init runs again (doc/guide/runtime.md#hot-reload).
//
// Reload touches the Lua state, so call it between ticks, never from inside a
// script call.
func (s *System) Reload() error { return s.reload() }

// reload compiles the file into a candidate and applies it on success. The
// file version is recorded either way, so a broken file is retried only after
// it changes.
func (s *System) reload() error {
	v, _ := s.src.stat() // before loading: an edit during the load triggers another reload
	s.loadedStat = v
	cand, err := s.compile()
	if err != nil {
		s.failedStat = v
		return err
	}
	s.loaded = *cand
	s.failedStat = fileVersion{}
	return nil
}

// compile loads and checks the script into a candidate without touching the
// running version.
func (s *System) compile() (*loaded, error) {
	rt := s.rt
	script, err := rt.load(s.src)
	if err != nil {
		return nil, err
	}

	key, ok := script.RawGetString("object").(lua.LString)
	if !ok {
		return nil, fmt.Errorf("%s: unknown object: the script must set object = \"group:class\"", s.src.name)
	}
	b, err := rt.binding(string(key))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.src.name, err)
	}
	update, ok := script.RawGetString("update").(*lua.LFunction)
	if !ok {
		return nil, fmt.Errorf("%s: update must be a function", s.src.name)
	}
	if init, ok := script.RawGetString("init").(*lua.LFunction); ok {
		rt.L.Push(init)
		if err := rt.L.PCall(0, 0, nil); err != nil {
			return nil, err
		}
	}

	c := &loaded{
		obj:    b,
		update: update,
		query:  rt.world.Query(b.requiredIDs()...),
		ref:    newObjectRef(b, rt.world),
		objUD:  rt.L.NewUserData(),
	}
	c.objUD.Value = c.ref
	rt.L.SetMetatable(c.objUD, b.meta)
	return c, nil
}

// watch reloads the script when the file changed, at most once per
// Options.WatchInterval of simulated time (the sum of ctx.Time.Delta, so the
// check costs nothing between intervals and needs no wall clock). A file that
// is momentarily missing (editors replace files on save) counts as unchanged.
// A bad version reports once; the previous version keeps running.
func (s *System) watch(ctx *system.Context) {
	interval := s.rt.opt.WatchInterval
	if interval <= 0 {
		return
	}
	s.elapsed += ctx.Time.Delta
	if s.elapsed < s.nextCheck {
		return
	}
	s.nextCheck = s.elapsed + interval

	v, err := s.src.stat()
	if err != nil || v == s.loadedStat || (v == s.failedStat && s.failedStat != fileVersion{}) {
		return
	}
	if err := s.reload(); err != nil {
		s.rt.fail(s.src.name, fmt.Errorf("reload failed, keeping previous version: %w", err))
	}
}

// Run calls update(dt, e, obj) for every entity that has all required
// components of the script's object.
func (s *System) Run(ctx *system.Context) {
	rt := s.rt
	L := rt.L
	rt.cmds = ctx.Commands
	defer func() { rt.cmds = nil }()

	s.watch(ctx)
	if s.update == nil {
		return
	}

	var budget *deadlineCtx
	if rt.opt.TickBudget > 0 {
		// Not context.WithTimeout: its timer goroutine never runs on
		// js/wasm while a script spins (see deadlineCtx).
		budget = newDeadlineCtx(rt.opt.TickBudget)
		L.SetContext(budget)
		defer L.RemoveContext()
	}

	dt := lua.LNumber(ctx.Time.Delta.Seconds())
	for e, ptrs := range s.query.Each() {
		s.point(e, ptrs)

		L.Push(s.update)
		L.Push(dt)
		L.Push(rt.entityUserdata(e)) // new each time: scripts may keep entity handles
		L.Push(s.objUD)
		if err := L.PCall(3, 0, nil); err != nil {
			if budget != nil && budget.Err() != nil {
				rt.fail(s.src.name, fmt.Errorf("%s: tick budget exceeded", s.src.name))
				return // every further call would fail the same way
			}
			rt.fail(s.src.name, err)
		}
	}
}

// point re-targets the reused object at e. ptrs holds one pointer per
// required component in query order; optional components are looked up on
// access, so their cached pointers are cleared.
func (s *System) point(e ecs.Entity, ptrs []any) {
	ref := s.ref
	ref.e = e
	clear(ref.ptr)
	for k, idx := range s.obj.required {
		ref.ptr[idx] = ptrs[k]
	}
}
