package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, data string, opts Options) (string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	opts.Out = &out
	opts.Err = &errb
	require.NoError(t, Render(json.RawMessage(data), opts))
	return out.String(), errb.String()
}

func TestRender_JSON(t *testing.T) {
	out, _ := render(t, `{"id":"KWN4QjAd","objective":"Go Engineer"}`, Options{Format: FormatJSON})
	assert.Contains(t, out, `"id": "KWN4QjAd"`)
	assert.Contains(t, out, `"objective": "Go Engineer"`)
}

func TestRender_YAML(t *testing.T) {
	out, _ := render(t, `{"id":"1","remote":true}`, Options{Format: FormatYAML})
	assert.Contains(t, out, "id:")
	assert.Contains(t, out, "remote: true")
}

func TestRender_CSV_DeterministicColumns(t *testing.T) {
	// id is a preferred key and must lead; the rest fall back to alphabetical.
	out, _ := render(t, `[{"objective":"B","id":"2"},{"objective":"A","id":"1"}]`, Options{Format: FormatCSV})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "id,objective", lines[0])
	assert.Equal(t, "2,B", lines[1])
}

func TestRender_CSV_FormulaInjectionGuarded(t *testing.T) {
	out, _ := render(t, `[{"name":"=cmd()","id":"1"}]`, Options{Format: FormatCSV, Columns: []string{"id", "name"}})
	assert.Contains(t, out, "'=cmd()", "leading = must be neutralized")
}

func TestRender_ID(t *testing.T) {
	out, _ := render(t, `[{"id":"a"},{"id":"b"}]`, Options{Format: FormatID})
	assert.Equal(t, "a\nb\n", out)
}

func TestRender_ID_PicksUsername(t *testing.T) {
	out, _ := render(t, `[{"username":"linkedinnegra","name":"x"}]`, Options{Format: FormatID})
	assert.Equal(t, "linkedinnegra\n", out)
}

func TestRender_Table(t *testing.T) {
	out, _ := render(t, `[{"id":"1","objective":"Go Engineer","remote":true}]`, Options{Format: FormatTable, NoColor: true})
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "OBJECTIVE")
	assert.Contains(t, out, "Go Engineer")
}

func TestRender_Table_SanitizesTerminalEscapes(t *testing.T) {
	// A name carrying an OSC terminal-escape (ESC ] 0 ; ... BEL) must be stripped in the
	// human table path. Build it via json.Marshal so the JSON is valid.
	b, _ := json.Marshal([]map[string]string{{"id": "1", "name": "\x1b]0;pwned\x07hi"}})
	out, _ := render(t, string(b), Options{Format: FormatTable, NoColor: true, Columns: []string{"id", "name"}})
	assert.NotContains(t, out, "\x1b]0;")
	assert.Contains(t, out, "hi")
}

func TestRender_JQ(t *testing.T) {
	out, _ := render(t, `{"person":{"name":"Alex"}}`, Options{Format: FormatJSON, JQ: ".person.name"})
	assert.Contains(t, out, "Alex")
}

func TestRender_JQ_Invalid(t *testing.T) {
	var out, errb bytes.Buffer
	err := Render(json.RawMessage(`{}`), Options{Format: FormatJSON, JQ: ".[", Out: &out, Err: &errb})
	require.Error(t, err)
}

func TestFormat_Valid(t *testing.T) {
	for _, f := range []Format{FormatTable, FormatJSON, FormatYAML, FormatCSV, FormatID} {
		assert.True(t, f.Valid())
	}
	assert.False(t, Format("xml").Valid())
}

func TestRender_UnknownFormat(t *testing.T) {
	err := Render(json.RawMessage(`{}`), Options{Format: Format("xml"), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	require.Error(t, err)
}

func TestRender_ColumnsCappedNote(t *testing.T) {
	// 12 keys, default cap 10 → a stderr note.
	obj := `[{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8,"i":9,"j":10,"k":11,"l":12}]`
	_, errb := render(t, obj, Options{Format: FormatCSV})
	assert.Contains(t, errb, "columns")
}

func TestSanitizeTerminal_FastPath(t *testing.T) {
	assert.Equal(t, "clean text", SanitizeTerminal("clean text"))
	assert.NotContains(t, SanitizeTerminal("\x1b[31mred\x1b[0m"), "\x1b")
}
