package stubs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

const healthGOD = `{"version":0,"components":[{"group":"g","name":"Health","goType":"g.Health","schema":{"type":"integer"}}],
"objects":[{"group":"g","class":"Hero","goType":"g.Hero","components":[{"component":"g:Health","field":"Health"}]}]}`

func TestLoad(t *testing.T) {
	sample := filepath.Join("testdata", "sample.god.json")
	tests := []struct {
		name  string
		files map[string]string // relative to a temp dir
		input []string          // relative to the temp dir; "" means sample
		want  []string          // error substrings; none means success
	}{
		{name: "same file twice", input: []string{"", ""}},
		{
			name:  "directory walk skips testdata",
			files: map[string]string{"a/x.god.json": healthGOD, "a/testdata/bad.god.json": "{"},
			input: []string{"a"},
		},
		{
			name: "identical definitions merge, docs ignored",
			files: map[string]string{
				"a.god.json": healthGOD,
				"b.god.json": strings.Replace(healthGOD, `"goType":"g.Health",`, `"goType":"g.Health","doc":"Other doc.",`, 1),
			},
			input: []string{"a.god.json", "b.god.json"},
		},
		{
			name: "conflicts and unknown components are reported together",
			files: map[string]string{
				"a.god.json": healthGOD,
				"b.god.json": strings.Replace(healthGOD, `"type":"integer"`, `"type":"number"`, 1),
				"c.god.json": `{"version":0,"objects":[{"group":"g","class":"Ghost","components":[{"component":"g:Missing","field":"M"}]}]}`,
			},
			input: []string{"a.god.json", "b.god.json", "c.god.json"},
			want:  []string{"component g:Health differs from the one in", "object g:Ghost uses component g:Missing, which no input defines"},
		},
		{
			name:  "bad inputs",
			files: map[string]string{"v.god.json": `{"version":1}`, "j.god.json": "{", "empty/readme.txt": ""},
			input: []string{"v.god.json", "j.god.json", "empty", "missing.god.json"},
			want:  []string{"unsupported version 1", "j.god.json: unexpected end of JSON input", "empty: no .god.json files", "missing.god.json"},
		},
		{name: "no inputs", input: []string{}, want: []string{"no GOD files given"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, data := range tt.files {
				writeFile(t, filepath.Join(dir, name), data)
			}
			var inputs []string
			for _, in := range tt.input {
				if in == "" {
					inputs = append(inputs, sample)
				} else {
					inputs = append(inputs, filepath.Join(dir, in))
				}
			}
			doc, err := load("T", inputs)
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if len(doc.Objects) == 0 {
					t.Error("no objects loaded")
				}
				return
			}
			if err == nil {
				t.Fatal("no error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error lacks %q:\n%v", w, err)
				}
			}
		})
	}
}
