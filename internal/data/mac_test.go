package data

import "testing"

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"88:A2:9E:34:27:61", "88:a2:9e:34:27:61"},
		{"88a29e342761", "88:a2:9e:34:27:61"},
		{"00-E0-4C-68-01-BE", "00:e0:4c:68:01:be"},
	}
	for _, tc := range tests {
		got, err := NormalizeMAC(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q => %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGatewayIDFromEth0MAC(t *testing.T) {
	id, err := GatewayIDFromEth0MAC("88:a2:9e:34:27:61")
	if err != nil {
		t.Fatal(err)
	}
	if id != "gw-88a29e342761" {
		t.Fatalf("got %q", id)
	}
}
