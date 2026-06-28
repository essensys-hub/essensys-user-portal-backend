package domain

import (
	"errors"
	"strings"
)

// User link modes (admin + portal access).
const (
	LinkModeArmoireDirect = "armoire"  // armoire polls OVH legacy IoT — portal cloud only
	LinkModeGateway       = "gateway"  // CM5 gateway — portal + mon.essensys.local
	LinkModeServerLegacy  = "server"   // essensys-server VPS — local legacy, no remote portal
)

// ResolveUserLinkMode infers the active link mode from persisted user fields.
func ResolveUserLinkMode(machineID *int, gatewayID *string, armoireID *int) string {
	if gatewayID != nil && strings.TrimSpace(*gatewayID) != "" {
		if !IsRemoteEligibleGateway(gatewayID) {
			return LinkModeServerLegacy
		}
		return LinkModeGateway
	}
	if machineID != nil || armoireID != nil {
		return LinkModeArmoireDirect
	}
	return ""
}

// UserPortalAccessEligible is true when the user may use mon.essensys.fr/portal after link approval.
func UserPortalAccessEligible(machineID *int, gatewayID *string, armoireID *int) bool {
	switch ResolveUserLinkMode(machineID, gatewayID, armoireID) {
	case LinkModeServerLegacy:
		return false
	case LinkModeGateway, LinkModeArmoireDirect:
		return machineID != nil
	default:
		return false
	}
}

// ValidateAdminUserLinks checks admin PUT /admin/users/{id}/links payloads.
func ValidateAdminUserLinks(machineID *int, gatewayID *string, armoireID *int) error {
	hasGW := gatewayID != nil && strings.TrimSpace(*gatewayID) != ""
	hasMachine := machineID != nil
	hasArmoire := armoireID != nil

	if !hasGW && !hasMachine && !hasArmoire {
		return nil
	}

	if hasGW && !IsRemoteEligibleGateway(gatewayID) {
		if hasMachine || hasArmoire {
			return errors.New(RemoteBlockedMessage())
		}
		return nil
	}

	if hasGW {
		if !hasMachine {
			return errors.New("serveur cloud (machine_id) requis avec une gateway CM5")
		}
		return nil
	}

	if !hasArmoire && !hasMachine {
		return errors.New("sélectionnez une armoire (inventaire OVH) pour le mode armoire seule")
	}
	return nil
}
