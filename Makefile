# Every contribution is a folder with its own go.mod. These targets run the
# same target in each of them.
MODULES = lua

.PHONY: all wasm-test modtidy
all wasm-test modtidy:
	@for m in $(MODULES); do $(MAKE) -C $$m $@ || exit 1; done
