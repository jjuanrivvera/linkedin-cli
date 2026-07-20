package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncCell(t *testing.T) {
	s, cut := truncCell("hello world", 5)
	assert.True(t, cut)
	assert.Equal(t, "hell…", s)
	s, cut = truncCell("hi", 10)
	assert.False(t, cut)
	assert.Equal(t, "hi", s)
}

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("-3.5"))
	assert.True(t, isNumeric("42"))
	assert.False(t, isNumeric("-abc"))
	assert.False(t, isNumeric(""))
	assert.False(t, isNumeric("1.2.3"))
}

func TestSanitizeCSV_NegativeNumberKept(t *testing.T) {
	assert.Equal(t, "-3", sanitizeCSV("-3"))
	assert.Equal(t, "'-cmd", sanitizeCSV("-cmd"))
	assert.Equal(t, "'@x", sanitizeCSV("@x"))
	assert.Equal(t, "'+x", sanitizeCSV("+x"))
	assert.Equal(t, "", sanitizeCSV(""))
}

func TestYAMLNormalize_Numbers(t *testing.T) {
	out, _ := render(t, `{"n":9007199254740993,"f":1.5,"s":"x"}`, Options{Format: FormatYAML})
	assert.Contains(t, out, "9007199254740993")
	assert.Contains(t, out, "1.5")
}

func TestScalarAndFlatten(t *testing.T) {
	// A bare scalar renders as itself in table mode.
	out, _ := render(t, `true`, Options{Format: FormatTable})
	assert.Equal(t, "true\n", out)
	// An array of scalars flattens to a "value" column.
	out2, _ := render(t, `["a","b"]`, Options{Format: FormatCSV})
	assert.Contains(t, out2, "value")
}

func TestCellOneLine(t *testing.T) {
	assert.Equal(t, "a b c", cellOneLine("a\tb\nc"))
	assert.Equal(t, "plain", cellOneLine("plain"))
}
