package ecslua

import (
	"bufio"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
)

// Options configures a [Runtime]. The zero value is usable.
type Options struct {
	// TickBudget is the wall-clock limit for one [System] Run. When it is
	// exceeded the running script is aborted, the error is reported once and
	// the rest of the tick is skipped. Zero means no limit.
	TickBudget time.Duration

	// OnError receives every script error: loader errors, Lua runtime errors,
	// schema validation failures and an exceeded TickBudget. The failing call
	// is abandoned and the tick continues with the next entity. The default
	// panics with "lua: " followed by the error text.
	OnError func(script string, err error)

	// WatchInterval turns on hot reload: each [System] checks its script file
	// at most once per interval of simulated time (the sum of the tick deltas)
	// and reloads it when it changed. A reload that fails is reported through
	// OnError once per file version, and the previous version keeps running.
	// Zero (the default) means no watching and no file-system call in Run.
	WatchInterval time.Duration

	// AllowCollectGarbage keeps the collectgarbage global, which the sandbox
	// removes by default.
	AllowCollectGarbage bool
}

// Runtime owns one Lua state bound to one World and hands out the [System]s
// that run scripts in it. It is not safe for concurrent use, like the World.
type Runtime struct {
	world *ecs.World
	L     *lua.LState
	opt   Options

	// cmds is the Commands buffer of the System.Run (or Init) in progress.
	// The world API queues structural changes on it. It is only valid while a
	// system call is running, as documented for system.Context.
	cmds *system.Commands

	entityMeta *lua.LTable
	metas      map[metaKey]*lua.LTable
	objects    map[string]*objectBinding
}

// NewRuntime creates a sandboxed Lua state for w and installs the `world` API
// (see doc/guide).
//
// Panics with "lua: nil world" if w is nil.
func NewRuntime(w *ecs.World, opt Options) *Runtime {
	if w == nil {
		panic("lua: nil world")
	}
	rt := &Runtime{
		world:   w,
		L:       lua.NewState(lua.Options{SkipOpenLibs: true}),
		opt:     opt,
		metas:   map[metaKey]*lua.LTable{},
		objects: map[string]*objectBinding{},
	}
	rt.openSandboxLibs()
	rt.entityMeta = newEntityMeta(rt.L)
	rt.installWorldAPI()
	return rt
}

// Close releases the Lua state. The Runtime must not be used afterwards.
func (rt *Runtime) Close() { rt.L.Close() }

// openSandboxLibs opens only the libraries scripts may use and strips the
// base functions that reach the file system or break script isolation.
func (rt *Runtime) openSandboxLibs() {
	L := rt.L
	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.MathLibName, lua.OpenMath},
		{lua.StringLibName, lua.OpenString},
		{lua.TabLibName, lua.OpenTable},
	} {
		L.Push(L.NewFunction(lib.open))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}

	globals := L.Get(lua.GlobalsIndex).(*lua.LTable)
	removed := []string{"dofile", "loadfile", "load", "loadstring", "getfenv", "setfenv", "module", "require"}
	if !rt.opt.AllowCollectGarbage {
		removed = append(removed, "collectgarbage")
	}
	for _, name := range removed {
		globals.RawSetString(name, lua.LNil)
	}
}

// load runs the script at path in its own environment and returns the table
// it returns. Globals the script sets stay in that environment; reads fall
// through to the sandbox globals (base functions, math, string, table, world).
func (rt *Runtime) load(src source) (*lua.LTable, error) {
	L := rt.L
	f, err := src.open()
	if err != nil {
		return nil, err
	}
	// Load (not LoadFile) so any fs.FS works; the chunk name is the script
	// name, which gives error positions like "scripts/x.lua:7:".
	fn, err := L.Load(bufio.NewReader(f), src.name)
	f.Close()
	if err != nil {
		return nil, err
	}

	env := L.NewTable()
	meta := L.NewTable()
	meta.RawSetString("__index", L.Get(lua.GlobalsIndex))
	L.SetMetatable(env, meta)
	L.SetFEnv(fn, env)

	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		return nil, err
	}
	ret := L.Get(-1)
	L.Pop(1)
	tbl, ok := ret.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s: script must return a table", src.name)
	}
	return tbl, nil
}

// fail reports a script error through OnError, or panics.
func (rt *Runtime) fail(script string, err error) {
	if rt.opt.OnError != nil {
		rt.opt.OnError(script, err)
		return
	}
	panic("lua: " + err.Error())
}

// commands returns the Commands buffer structural changes are queued on.
func (rt *Runtime) commands() *system.Commands {
	if rt.cmds != nil {
		return rt.cmds
	}
	return system.CommandsOf(rt.world)
}
