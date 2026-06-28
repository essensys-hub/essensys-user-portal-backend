package domain

import "testing"

func TestResolveUserLinkMode(t *testing.T) {
	gw := "gw-essensys-gateway"
	server := "essensys-server"
	machine := 29
	armoire := 29

	if got := ResolveUserLinkMode(&machine, &gw, &armoire); got != LinkModeGateway {
		t.Fatalf("gateway mode: got %q", got)
	}
	if got := ResolveUserLinkMode(nil, &server, nil); got != LinkModeServerLegacy {
		t.Fatalf("server mode: got %q", got)
	}
	if got := ResolveUserLinkMode(&armoire, nil, &armoire); got != LinkModeArmoireDirect {
		t.Fatalf("armoire direct: got %q", got)
	}
}

func TestUserPortalAccessEligible(t *testing.T) {
	gw := "gw-essensys-gateway"
	server := "essensys-server"
	machine := 19

	if !UserPortalAccessEligible(&machine, &gw, nil) {
		t.Fatal("gateway+machine should have portal access")
	}
	if UserPortalAccessEligible(&machine, &server, nil) {
		t.Fatal("essensys-server should not have remote portal")
	}
	if !UserPortalAccessEligible(&machine, nil, &machine) {
		t.Fatal("armoire direct with machine_id should have portal access")
	}
}
