package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanSecretLine(t *testing.T) {
	got, err := scanSecretLine(strings.NewReader("secret-token\nmore"))
	require.NoError(t, err)
	assert.Equal(t, "secret-token", got)
}

func TestScanSecretLine_Backspace(t *testing.T) {
	got, err := scanSecretLine(strings.NewReader("abx\x7f\n"))
	require.NoError(t, err)
	assert.Equal(t, "ab", got)
}

func TestScanSecretLine_CtrlC(t *testing.T) {
	_, err := scanSecretLine(strings.NewReader("ab\x03"))
	require.Error(t, err)
}

func TestScanSecretLine_EOFNoNewline(t *testing.T) {
	got, err := scanSecretLine(strings.NewReader("token"))
	require.NoError(t, err)
	assert.Equal(t, "token", got)
}

func TestSanitizeSecret(t *testing.T) {
	assert.Equal(t, "abc", sanitizeSecret("\x1b[200~abc\x1b[201~"))
	assert.Equal(t, "abc", sanitizeSecret("  abc  "))
}
