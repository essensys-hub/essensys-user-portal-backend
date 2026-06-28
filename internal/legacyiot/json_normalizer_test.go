package legacyiot

import "testing"

func TestNormalizeJSON_BasicMalformedJSON(t *testing.T) {
	input := []byte(`{version:"1.0",ek:[{k:613,v:"64"},{k:607,v:"0"}]}`)
	result, err := NormalizeJSON(input)
	if err != nil {
		t.Fatalf("NormalizeJSON failed: %v", err)
	}
	if string(result) == string(input) {
		t.Fatal("expected normalized JSON")
	}
}
