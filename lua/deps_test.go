package ecslua_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestEngineModuleDependencies enforces the module rule of the Lua module:
// it, its tests included, uses only the exported god package of the weakpixel engine
// module github.com/weakpixel/engine (and the Weakpixel ECS module
// github.com/weakpixel/ecs). Game Objects for tests are its own
// (internal/luatest), never generated output of another module.
func TestEngineModuleDependencies(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found")
	}
	const engine = "github.com/weakpixel/engine/"
	out, err := exec.Command("go", "list", "-deps", "-test", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps -test ./...: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if !strings.HasPrefix(dep, engine) {
			continue
		}
		if dep != engine+"god" {
			t.Errorf("the lua module depends on %s; it may use only god of the weakpixel engine module", dep)
		}
	}
}
