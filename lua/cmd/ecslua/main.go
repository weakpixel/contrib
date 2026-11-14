// Command ecslua is the command line of the Lua scripting module.
//
//	ecslua stubs [-o file] [-title name] <GOD files or directories>...
//
// stubs writes one LuaLS definition file for the Game Objects and components
// of the GOD files, so editors complete and check scripts; see
// doc/guide/editor-support.md.
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

const usage = "usage: ecslua stubs [-o file] [-title name] <GOD files or directories>..."

// run executes the command line args and returns the exit code: 0 on
// success, 1 when the inputs are bad, 2 on a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "stubs" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("stubs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}
	out := fs.String("o", "-", "output file, - for standard output")
	title := fs.String("title", "Game", "project name in the file header")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}

	text, err := stubs.Generate(*title, fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, "ecslua stubs:", err)
		return 1
	}
	if *out == "-" {
		_, err = io.WriteString(stdout, text)
	} else {
		if err = os.MkdirAll(filepath.Dir(*out), 0o755); err == nil {
			err = os.WriteFile(*out, []byte(text), 0o644)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "ecslua stubs:", err)
		return 1
	}
	return 0
}
