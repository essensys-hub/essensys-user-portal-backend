package geo

import "testing"

func TestIsLookupable(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"", false},
		{"127.0.0.1", false},
		{"192.168.0.14", false},
		{"10.0.0.1", false},
		{"203.0.113.10", true},
		{"203.0.113.10:443", true},
	}
	for _, tc := range cases {
		if got := IsLookupable(tc.ip); got != tc.want {
			t.Fatalf("IsLookupable(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestLookupIP_skipsPrivate(t *testing.T) {
	if _, err := LookupIP("192.168.1.1"); err == nil {
		t.Fatal("expected error for private IP")
	}
}
