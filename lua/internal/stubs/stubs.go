package stubs

// Generate reads the GOD files named by inputs (files, or directories
// searched recursively for *.god.json) and returns one LuaLS meta file for
// them; title names the project in its header. Every problem of the inputs
// (unreadable or bad file, unsupported version, conflicting definitions, an
// object using an undefined component) is reported together.
func Generate(title string, inputs []string) (string, error) {
	doc, err := load(title, inputs)
	if err != nil {
		return "", err
	}
	return renderLua(doc), nil
}
