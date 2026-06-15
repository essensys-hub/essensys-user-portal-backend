package domain

import "testing"

func TestIsRemoteEligibleGateway(t *testing.T) {
	gw := "gw-essensys-gateway"
	if !IsRemoteEligibleGateway(&gw) {
		t.Fatal("CM5 gateway should be eligible")
	}
	server := "essensys-server"
	if IsRemoteEligibleGateway(&server) {
		t.Fatal("essensys-server must not be remote eligible")
	}
	prefixed := "gw-essensys-server"
	if IsRemoteEligibleGateway(&prefixed) {
		t.Fatal("gw-essensys-server must not be remote eligible")
	}
	empty := ""
	if IsRemoteEligibleGateway(&empty) {
		t.Fatal("empty gateway ID must not be remote eligible")
	}
	if IsRemoteEligibleGateway(nil) {
		t.Fatal("nil gateway ID must not be remote eligible")
	}
}
