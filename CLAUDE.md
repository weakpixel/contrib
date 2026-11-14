# contrib

Optional add-on modules for the Weakpixel Engine (repository weakpixel/engine). Every contribution is a folder with its own `go.mod` (module `github.com/weakpixel/contrib/<name>`), README, docs and tests. The engine's code, docs and rules live in the engine repository; read its CLAUDE.md and `doc/` for anything about the engine.

| Path | What |
| --- | --- |
| `lua/` | Module `github.com/weakpixel/contrib/lua`, package `ecslua`: Lua (GopherLua) systems driven by GOD schemas. `internal/stubs` and `cmd/ecslua` write LuaLS stubs; `internal/luatest` and `example/game` are test Game Objects generated with `weakpixel gen` |

## The docs are the spec

Each module's `doc/` is its spec: `doc/guide/` for users, `doc/design.md` for how and why. If code and doc disagree, don't silently pick one: fix the code, or update the doc in the same change and say so. Panic and error texts in the guide are API. Every topic has one home; link to it instead of repeating it. The `doc-writer` skill covers any documentation change.

## Commands

```bash
make all         # every module: go generate, fmt, vet, test -race
make wasm-test   # every module under GOOS=js GOARCH=wasm (Node)
make -C lua bench
```

Run `make all` and `make wasm-test` once before opening a PR; CI runs both and fails on a generate or gofmt diff. While iterating, run only the tests of the package you changed. The engine repository is private for now: `GOPRIVATE=github.com/weakpixel/engine` (the Makefiles export it).

## Rules

- **Engine changes go to weakpixel/engine first.** A contribution that needs an engine change waits for the engine PR to merge, then bumps `github.com/weakpixel/engine` in its `go.mod`. Work on both with an untracked `go.work`, never a committed `replace`. The engine is pinned at a pseudo-version of its `main` until it is tagged.
- **Nothing in the engine depends on a contribution.** The engine links to them in docs only.
- **One module per folder, no shared code between modules.** A new contribution is added to `MODULES` in the root `Makefile` and to the table in `README.md`.
- **Lua module:** from the engine module it may import only `god` (`deps_test.go` enforces it, so no Ebiten in its dependency list); no cgo, it must build for js/wasm.
- **Never hand-edit a generated file** (`zz__gen.go`, GOD files): run `go generate ./...`.
- **Build outputs go into `build/`** (`go build -o build/<name>`), which is ignored. Never commit binaries or `*.test` files. Before every commit run `git status --short` and `git diff --cached --stat`, and stage files by name, not with `git add -A` or `git add .`.
- **Versions stay 0.** No tag or release without the maintainer's word (module-folder tags such as `lua/v0.0.1`), and no backward compatibility shims before a release.
- **No attribution and no names:** no Co-Authored-By, Claude-Session or "Generated with" lines in commits, PRs, docs or code; never mention a person's name in any document, commit or PR; no hidden HTML markers or comments anywhere. Branch names, tags, commits and PR titles never contain "claude".

## Code quality

- **Readability first.** Clear names, small functions, straightforward control flow.
- **Document the public API.** Every exported identifier has a doc comment that states what it does, its contract (goroutine, ownership, cost) and which errors or panics it can raise.
- **Explain performance tricks inline**, so a reader does not "simplify" them away.
- Tests use the standard `testing` package only (no testify), table-driven with `t.Run`. File names are kebab-case. Modern Go: `iter`, `slices`, `maps`, `for i := range n`.

## Workflow

Non-trivial work goes through `/plan` (writes one plan file, no code) and `/implement` (carries it out step by step and keeps the plan file up to date). Small, obvious requests don't need a plan.

| Skill | Use when |
| --- | --- |
| `plan` | User runs `/plan` |
| `implement` | User runs `/implement` |
| `doc-writer` | Any documentation change |
