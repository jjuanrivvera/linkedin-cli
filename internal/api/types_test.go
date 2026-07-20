package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID(t *testing.T) {
	var s struct {
		A ID `json:"a"`
		B ID `json:"b"`
		C ID `json:"c"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a":"x1","b":4012345678,"c":null}`), &s))
	assert.Equal(t, ID("x1"), s.A)
	assert.Equal(t, ID("4012345678"), s.B)
	assert.Equal(t, ID(""), s.C)

	out, _ := json.Marshal(ID("42"))
	assert.Equal(t, `"42"`, string(out))
}

func TestID_Invalid(t *testing.T) {
	var id ID
	assert.Error(t, id.UnmarshalJSON([]byte(`{bad}`)))
}

func TestInt(t *testing.T) {
	var s struct {
		A Int `json:"a"`
		B Int `json:"b"`
		C Int `json:"c"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a":8000,"b":"12","c":null}`), &s))
	assert.Equal(t, int64(8000), s.A.Int64())
	assert.Equal(t, int64(12), s.B.Int64())
	assert.Equal(t, int64(0), s.C.Int64())

	var bad Int
	assert.Error(t, bad.UnmarshalJSON([]byte(`"NaN"`)))
}

func TestBool(t *testing.T) {
	var s struct {
		A Bool `json:"a"`
		B Bool `json:"b"`
		C Bool `json:"c"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a":true,"b":"yes","c":"nope"}`), &s))
	assert.True(t, bool(s.A))
	assert.True(t, bool(s.B))
	assert.False(t, bool(s.C))
}

func TestStringOrSlice(t *testing.T) {
	var s struct {
		A StringOrSlice `json:"a"`
		B StringOrSlice `json:"b"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a":"one","b":["x","y"]}`), &s))
	assert.Equal(t, StringOrSlice{"one"}, s.A)
	assert.Equal(t, StringOrSlice{"x", "y"}, s.B)
}

func FuzzFlexibleTypes(f *testing.F) {
	f.Add(`"x"`)
	f.Add(`123`)
	f.Add(`null`)
	f.Add(`["a","b"]`)
	f.Add(`true`)
	f.Fuzz(func(_ *testing.T, s string) {
		var id ID
		_ = id.UnmarshalJSON([]byte(s))
		var i Int
		_ = i.UnmarshalJSON([]byte(s))
		var b Bool
		_ = b.UnmarshalJSON([]byte(s))
		var ss StringOrSlice
		_ = ss.UnmarshalJSON([]byte(s))
	})
}
