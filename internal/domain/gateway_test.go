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

func TestValidateNoPortalLinkRemoval(t *testing.T) {
	gw := "gw-essensys-gateway"
	machine := 19
	target := &User{LinkedGatewayID: &gw, LinkedMachineID: &machine}

	if err := ValidateNoPortalLinkRemoval(target, &machine, &gw); err != nil {
		t.Fatalf("update with values should pass: %v", err)
	}
	if err := ValidateNoPortalLinkRemoval(target, &machine, nil); err == nil {
		t.Fatal("clearing gateway should fail")
	}
	if err := ValidateNoPortalLinkRemoval(target, nil, &gw); err == nil {
		t.Fatal("clearing machine should fail")
	}

	empty := &User{}
	if err := ValidateNoPortalLinkRemoval(empty, nil, nil); err != nil {
		t.Fatalf("user without gateway should allow first link: %v", err)
	}
}
