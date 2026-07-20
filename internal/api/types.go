package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// LinkedIn's Voyager JSON mixes id shapes — a jobPostingId is a number (4012345678), a
// company staffCount is a number, some ids arrive as numeric strings. These flexible JSON types
// decode any of those without breaking, per the cliwright standard (§2). Unknown fields are
// ignored so structs need not be exhaustive.

// ID unmarshals from a JSON string OR number and always marshals as a string, so ids render
// consistently and never lose precision above 2^53.
type ID string

func (id *ID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*id = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*id = ID(s)
		return nil
	}
	if !isJSONNumber(b) {
		return fmt.Errorf("ID: %q is neither a string nor a number", string(b))
	}
	*id = ID(string(b))
	return nil
}

func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(string(id)) }

func (id ID) String() string { return string(id) }

// Int accepts a JSON number OR a numeric string, decoding int64 before float64 so ids above
// 2^53 keep precision. NaN/Inf and malformed numbers are rejected.
type Int int64

func (n *Int) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*n = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*n = 0
			return nil
		}
		b = []byte(s)
	}
	if !isJSONNumber(b) {
		return fmt.Errorf("Int: invalid number %q", string(b))
	}
	if i, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		*n = Int(i)
		return nil
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("Int: %w", err)
	}
	*n = Int(int64(f))
	return nil
}

func (n Int) Int64() int64 { return int64(n) }

// Bool accepts a real bool OR "true"/"1"/"yes". LinkedIn's workRemoteAllowed is a real bool, but
// some flags arrive as strings.
type Bool bool

func (v *Bool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*v = false
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		switch s {
		case "true", "True", "TRUE", "1", "yes", "Yes":
			*v = true
		default:
			*v = false
		}
		return nil
	}
	var raw bool
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("Bool: %w", err)
	}
	*v = Bool(raw)
	return nil
}

// StringOrSlice accepts a single string OR an array of strings — LinkedIn's workplaceTypes is an
// array, but adjacent fields are sometimes a scalar.
type StringOrSlice []string

func (s *StringOrSlice) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*s = nil
		return nil
	}
	if b[0] == '[' {
		var arr []string
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var one string
	if err := json.Unmarshal(b, &one); err != nil {
		return fmt.Errorf("StringOrSlice: %w", err)
	}
	*s = []string{one}
	return nil
}

// isJSONNumber reports whether b matches the JSON number grammar (rejects NaN/Inf/garbage).
func isJSONNumber(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	var f float64
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&f); err != nil {
		return false
	}
	return !dec.More()
}
