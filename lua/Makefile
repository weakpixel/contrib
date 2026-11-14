# The Lua module; run these targets from this directory, or from the
# repository root as `make lua` (= `make -C lua all`).
# GOPRIVATE=github.com/weakpixel/engine while the engine repository is private.

.PHONY: all
all:
	go generate ./...
	go fmt ./...
	go vet ./...
	go test ./...

.PHONY: modtidy
modtidy:
	go mod tidy
	
# Runs the example: `make run`, or with flags: `make run ARGS="-ticks 20 -dt 0.25"`.
ARGS ?=
.PHONY: run
run: all
	go run ./example $(ARGS)

.PHONY: bench
bench:
	go test -run '^$$' -bench . -benchmem .

# All generated GOD and documentation outputs live below the ignored build/.
EXAMPLE_GOD = build/god/example.god.json

.PHONY: example-god
example-god:
	go generate ./example/game

# LuaLS stubs of the example's Game Objects (doc/guide/editor-support.md).
.PHONY: stubs
stubs: example-god
	go run ./cmd/ecslua stubs -title Example -o ./build/god-doc/example.lua $(EXAMPLE_GOD)

# Optional documentation of the example's Game Objects and Components.
# Generated files go under the ignored build directory.
.PHONY: god-doc
god-doc: example-god
	go tool weakpixel doc -o ./build/god-doc/example.html $(EXAMPLE_GOD)
	go tool weakpixel doc -o ./build/god-doc/example.md $(EXAMPLE_GOD)


# js/wasm. WASM_ENV runs go test under go_js_wasm_exec (Node): it puts the
# exec wrapper on PATH and starts from an empty environment, because Node
# refuses a command line plus environment over its size limit, which a
# large CI or container environment exceeds.
WASM_ENV = env -i HOME="$(HOME)" PATH="$(shell go env GOROOT)/lib/wasm:$(PATH)" \
	GOPATH="$(shell go env GOPATH)" GOMODCACHE="$(shell go env GOMODCACHE)" \
	GOCACHE="$(shell go env GOCACHE)" GOTOOLCHAIN="$(shell go env GOTOOLCHAIN)" \
	GOFLAGS=-mod=mod GOOS=js GOARCH=wasm

.PHONY: wasm-build
wasm-build:
	GOOS=js GOARCH=wasm go build ./...

.PHONY: wasm-test
wasm-test:
	@command -v node >/dev/null || { echo "wasm-test: node not found, skipped"; exit 0; }
	$(WASM_ENV) go test ./...
