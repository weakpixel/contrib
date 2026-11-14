// Command ecslua is the command-line tool of the Lua module.
//
// Usage:
//
//	go tool ecslua stubs [-o file] [-title text] [inputs...]
//
// stubs writes one LuaLS meta file ("definition file") for the Game Objects and
// Components of GOD files, so the Lua language server completes and checks
// scripts (doc/guide/editor-support.md). Inputs are GOD files (*.god.json),
// directories (searched recursively) or patterns such as ./... and dir/...;
// the default is build/god. The output defaults to build/god.lua; "-" is stdout.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/weakpixel/contrib/lua/internal/stubs"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command and returns the exit code: 0 on success, 1 for a
// failure, 2 for a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "stubs" {
		fmt.Fprintln(stderr, "usage: ecslua stubs [-o file] [-title text] [inputs...]")
		return 2
	}
	fs := flag.NewFlagSet("ecslua stubs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", "build/god.lua", `output file ("-" is stdout)`)
	title := fs.String("title", "Game Objects", "title in the header of the file")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	inputs := fs.Args()
	if len(inputs) == 0 {
		inputs = []string{"build/god"}
	}

	lua, err := stubs.Render(*title, inputs)
	if err != nil {
		fmt.Fprintln(stderr, "ecslua:", err)
		return 1
	}
	if *output == "-" {
		io.WriteString(stdout, lua)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err == nil {
		err = os.WriteFile(*output, []byte(lua), 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "ecslua:", err)
		return 1
	}
	return 0
}
