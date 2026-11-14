module github.com/weakpixel/contrib/lua

go 1.27

tool (
	github.com/weakpixel/contrib/lua/cmd/ecslua
	github.com/weakpixel/engine/tools/weakpixel
)

require (
	github.com/weakpixel/ecs v0.0.1
	github.com/weakpixel/engine v0.0.0-20261114114223-f4b9f1675910
	github.com/yuin/gopher-lua v1.1.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/weakpixel/asset v0.0.1 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)
