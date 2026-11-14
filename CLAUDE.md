# Weakpixel contrib

Optional add-ons to the Weakpixel Engine (module `github.com/weakpixel/engine`, repository weakpixel/engine), MIT. Each contribution is a folder with its own `go.mod`, `Makefile`, `README.md`, `doc/` and tests; the engine never depends on them. The engine's code, docs and rules live in the engine repository.

| Folder | Module | What |
| --- | --- | --- |
| `lua/` | `github.com/weakpixel/contrib/lua` | Package `ecslua`: Lua (GopherLua) systems driven by the GOD schemas; `cmd/ecslua stubs` writes LuaLS stubs; docs in `lua/doc/` |

## Commands

```bash
make all          # every contribution: generate, fmt, vet, test
make wasm-test    # every contribution under GOOS=js GOARCH=wasm (Node)
make -C lua all   # one contribution; run it while iterating
```

The engine repository is private for now: `GOPRIVATE=github.com/weakpixel/engine` (the root Makefile exports it). CI runs every folder with a `go.mod` (native and wasm) and fails on a generate or gofmt diff.

## Rules

- **One folder, one module.** A new contribution gets its own folder, `go.mod` (module `github.com/weakpixel/contrib/<folder>`), `Makefile` with `all` and `wasm-test`, README, docs, tests and `THIRD_PARTY_NOTICES.md`, and a row in the root `README.md` and here. Contributions do not import each other.
- **Engine changes go to weakpixel/engine first.** A contribution that needs an engine change waits for the engine PR to merge, then bumps `github.com/weakpixel/engine` in its `go.mod`. Work on both with an untracked `go.work`, never a committed `replace`.
- **Small engine surface.** Each contribution states which engine packages it may import and a `deps_test.go` enforces it (Lua: `god` only).
- **Link engine docs with absolute GitHub URLs**, never relative paths out of the repository.
- **Never hand-edit a generated file** (`zz__gen.go`, GOD files): run `go generate ./...`.
- **Build outputs go into `build/`** of the contribution (`go build -o build/<name>`); `build/` is ignored. Never commit binaries, `*.test` or `.wasm` files. Before every commit run `git status --short` and `git diff --cached --stat`, and stage files by name.
- **Versions stay 0.** Tags are per folder (`lua/v0.0.1`) and only on the maintainer's word; no backward compatibility shims before a release.
- **No attribution and no names:** no Co-Authored-By, Claude-Session or "Generated with" lines in commits, PRs, docs or code; never mention a person's name in any document, commit or PR; no hidden HTML markers or comments anywhere.

## Code quality

- Readability first; document every exported identifier (what it does, contract, errors and panics).
- Tests use the standard `testing` package only, table-driven with `t.Run`.
- File names are kebab-case. Use modern Go (range-over-func, `for i := range n`, `slices`, `maps`).

## Workflow

Non-trivial work goes through `/plan` (writes one plan file, no code) and `/implement` (carries it out and keeps the plan file up to date). Small, obvious requests don't need a plan.

| Skill | Use when |
| --- | --- |
| `plan` | User runs `/plan` |
| `implement` | User runs `/implement` |
| `doc-writer` | Any documentation change |
