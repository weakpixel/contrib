# Every folder with a go.mod is a contribution with its own Makefile
# (targets all and wasm-test). GOPRIVATE: the engine repository is private
# for now.
export GOPRIVATE ?= github.com/weakpixel/engine

MODULES := $(patsubst %/go.mod,%,$(wildcard */go.mod))

.PHONY: all wasm-test $(MODULES)
all:
	@set -e; for m in $(MODULES); do $(MAKE) -C $$m all; done

wasm-test:
	@set -e; for m in $(MODULES); do $(MAKE) -C $$m wasm-test; done

$(MODULES):
	$(MAKE) -C $@ all
