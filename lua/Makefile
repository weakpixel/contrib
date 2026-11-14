# Generate, format, vet and test with the race detector.
.PHONY: all
all:
	go generate ./...
	go fmt ./...
	go vet ./...
	go test -race ./...

.PHONY: modtidy
modtidy:
	go mod tidy

# Runs the example: `make run`, or with flags: `make run ARGS="-ticks 20 -dt 0.25"`.
ARGS ?=
.PHONY: run
run:
	go run ./example $(ARGS)

# Per-entity cost of a Lua system.
.PHONY: bench
bench:
	go test -run '^$$' -bench . -benchmem .

# LuaLS stubs of the example's Game Objects, written to build/example.lua.
.PHONY: stubs
stubs:
	go generate ./example/game
	go tool ecslua stubs -o build/example.lua build/god/example.god.json

# js/wasm. WASM_ENV runs go test under go_js_wasm_exec (Node): it puts the
# exec wrapper on PATH and starts from an empty environment, because Node
# refuses a command line plus environment over its size limit, which a
# large CI or container environment exceeds.
WASM_ENV = env -i HOME="$(HOME)" PATH="$(shell go env GOROOT)/lib/wasm:$(PATH)" \
	GOPATH="$(shell go env GOPATH)" GOMODCACHE="$(shell go env GOMODCACHE)" \
	GOCACHE="$(shell go env GOCACHE)" GOTOOLCHAIN="$(shell go env GOTOOLCHAIN)" \
	GOFLAGS=-mod=mod GOOS=js GOARCH=wasm

.PHONY: wasm-test
wasm-test:
	@command -v node >/dev/null || { echo "wasm-test: node not found"; exit 1; }
	$(WASM_ENV) go vet ./...
	$(WASM_ENV) go test ./...
