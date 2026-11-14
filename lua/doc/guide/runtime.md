# Runtime (Go side)

```go
import ecslua "github.com/weakpixel/contrib/lua"

rt := ecslua.NewRuntime(world, ecslua.Options{})
defer rt.Close()
sys := rt.NewSystem("scripts/regen.lua") // *ecslua.System, a system.Runner + Initializer
```

| API | |
| --- | --- |
| `NewRuntime(w, opt) *Runtime` | One sandboxed Lua state for one World. Not safe for concurrent use (the World is single-goroutine too). Panics `lua: nil world`. |
| `(*Runtime).Close()` | Release the Lua state. Don't use the runtime afterwards. |
| `(*Runtime).NewSystem(path) *System` | A system backed by an OS file. A `system.Runner` and `system.Initializer`: the file is loaded and checked in `Init`, so mistakes surface before the first tick. |
| `(*Runtime).NewSystemFS(fsys, name) *System` | Same, from an `fs.FS`. |
| `(*System).Reload() error` | Load now, swap in on success. |

All Game Objects must be registered (`types` schemas) before the script loads.

## Options

| Option | Default | Meaning |
| --- | --- | --- |
| `TickBudget` | 0 (none) | Wall-clock limit for one system `Run`. When exceeded the running script is aborted with a Lua error, reported once, and the rest of the tick is skipped. The VM checks the clock every 1024 instructions without a timer goroutine, so the budget also works on js/wasm (one thread). |
| `OnError` | panic `lua: <script>: <err>` | Receives every script error. Set it to log and keep the game running. |
| `WatchInterval` | 0 (off) | Enables [hot reload](#hot-reload): each system checks its script at most once per interval of simulated time. |
| `AllowCollectGarbage` | false | Keep the `collectgarbage` global. |

```go
rt := ecslua.NewRuntime(world, ecslua.Options{
	TickBudget: 2 * time.Millisecond,
	OnError:    func(script string, err error) { log.Printf("%s: %v", script, err) },
})
```

## Scripts from an `fs.FS`

```go
//go:embed scripts/*.lua
var scripts embed.FS
s.AddRunner(rt.NewSystemFS(scripts, "scripts/regen.lua"))
```

`name` must satisfy `fs.ValidPath` (slash-separated, no leading `/`, no `..`; else panic `lua: invalid script name <name>`; `lua: nil fs` for a nil FS). It is the script name in errors and in `OnError`. Everything else, including the script contract and errors, is identical to a file-backed system.

Hot reload works through `fs.Stat`: it reloads when the FS reports a different modtime or size (`os.DirFS`, `fstest.MapFS`, zip file systems). An `embed.FS` reports a constant version, so its scripts never change and `WatchInterval` does nothing; `Reload()` re-reads the same bytes.

## Hot reload

Edit a script while the game runs.

```go
rt := ecslua.NewRuntime(world, ecslua.Options{WatchInterval: time.Second}) // dev builds
// or by hand, between ticks:
if err := sys.Reload(); err != nil { log.Print(err) }
```

- **Watching:** `System.Run` stats the script when at least one interval of *simulated* time has passed (the sum of `ctx.Time.Delta`; there is no wall-clock call). If the modtime or size changed, the script is reloaded before that tick's `update` calls, so the swap always happens between ticks. With `WatchInterval == 0`, `Run` makes no file-system call. A file that is briefly missing (editors replace files on save) counts as unchanged.
- **Atomic:** the new version must pass all `Init` checks (returns a table, `object` registered, `update` a function, `init` runs). Otherwise the old version keeps running and `reload failed, keeping previous version: ...` goes to `OnError` once per file version. `Reload()` returns the error instead.
- **Recovery:** a system whose first load failed is disabled and retries when the file changes.
- **State resets:** a fresh environment each reload: script locals and globals reset, `init` runs again. Entity data is untouched.
- **`object` may change** between versions: the query is rebuilt, so a `game:Hero` script can become a `game:Boss` script.
- `Reload()` touches the Lua state: call it between ticks, never from inside a script call.
- Only the script file is watched: not directories, not Go code, not the GOD schema.

## Sandbox

Not available: `os`, `io`, `debug`, `package`, `dofile`, `loadfile`, `load`, `loadstring`, `getfenv`, `setfenv`, `module`, `require` (and `collectgarbage` unless allowed). `math`, `string`, `table` and the base functions are.

GopherLua has **no instruction or memory limit**: `TickBudget` bounds wall-clock time only, and a script can still allocate without bound.

## Errors

| Condition | Message |
| --- | --- |
| script missing or not Lua | loader error (contains the file name) |
| doesn't return a table | `<script>: script must return a table` |
| `object` unset or unregistered | `<script>: unknown object <key>` |
| `update` not a function | `<script>: update must be a function` |
| unknown field | `unknown field <Comp>.<name>` |
| unknown component | `unknown component <g:C>.<name>` |
| write to readonly | `component <name> is readonly` |
| assign a whole component | `cannot assign component <name>: set its fields, or use world.set` |
| bad value | `<Comp>: $.<path>: expected <type>, got <value>` |
| stale entity | `stale entity` |
| reload failed | `reload failed, keeping previous version: <error>` |
| budget exceeded | `<script>: tick budget exceeded` |

The table shows messages without the `lua: ` prefix and without the script position. The default `OnError` prefixes `lua: ` and panics; set `OnError` to log and keep the game running.

## Performance

Lua is not part of the 0-alloc hot path. Per entity: one Lua call plus boxing of the arguments and of every number read, and a fresh entity handle (scripts may keep it). The object and component userdata are reused, so they cost nothing per entity; nested structs (`hero.Hitbox`) and `world.get` allocate.

About 2 µs and 21 allocations per entity for a script with three field reads and two writes: roughly 2 ms per tick for 1,000 entities and 24 ms for 10,000 (darwin/amd64, 2026-10-05). `make bench` (`BenchmarkLuaSystem`) measures it on your machine.

## Not included

Parallel scripts, coroutine state machines spanning ticks, generated typed bindings.
