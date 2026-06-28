package legacyiot

import (
	"encoding/json"
	"regexp"
)

// NormalizeJSON converts malformed JSON from legacy firmware to valid JSON.
// The C client sends unquoted keys: {version:"1.0",ek:[{k:123,v:"1"}]}
func NormalizeJSON(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, json.Unmarshal(input, new(interface{}))
	}

	normalized := string(input)
	normalized = regexp.MustCompile(`\{k:`).ReplaceAllString(normalized, `{"k":`)
	normalized = regexp.MustCompile(`,v:`).ReplaceAllString(normalized, `,"v":`)
	normalized = regexp.MustCompile(`\[k:`).ReplaceAllString(normalized, `[{"k":`)
	normalized = regexp.MustCompile(`\{version:`).ReplaceAllString(normalized, `{"version":`)
	normalized = regexp.MustCompile(`,version:`).ReplaceAllString(normalized, `,"version":`)
	normalized = regexp.MustCompile(`,ek:`).ReplaceAllString(normalized, `,"ek":`)

	var test interface{}
	if err := json.Unmarshal([]byte(normalized), &test); err != nil {
		return nil, err
	}
	return []byte(normalized), nil
}
