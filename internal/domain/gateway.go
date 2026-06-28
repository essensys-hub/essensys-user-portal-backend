package domain

import (
	"errors"
	"strings"
)

// RemoteIneligibleGatewayHost — pas de portail distant mon.essensys.fr (VPS legacy).
const RemoteIneligibleGatewayHost = "essensys-server"

func NormalizeGatewayHost(gatewayID string) string {
	g := strings.TrimSpace(strings.ToLower(gatewayID))
	return strings.TrimPrefix(g, "gw-")
}

func IsRemoteEligibleGateway(gatewayID *string) bool {
	if gatewayID == nil || *gatewayID == "" {
		return false
	}
	return NormalizeGatewayHost(*gatewayID) != RemoteIneligibleGatewayHost
}

// RemoteBlockedMessage is shown in admin UI when linking essensys-server to remote portal.
func RemoteBlockedMessage() string {
	return "essensys-server ne supporte pas le portail distant mon.essensys.fr — liaison armoire et serveur cloud interdites"
}

// ValidateNoPortalLinkRemoval rejects clearing gateway or cloud machine once linked.
func ValidateNoPortalLinkRemoval(target *User, machineID *int, gatewayID *string) error {
	if target == nil || target.LinkedGatewayID == nil || strings.TrimSpace(*target.LinkedGatewayID) == "" {
		return nil
	}
	if gatewayID == nil || strings.TrimSpace(*gatewayID) == "" {
		return errors.New("la gateway ne peut pas être retirée une fois liée")
	}
	if IsRemoteEligibleGateway(target.LinkedGatewayID) && target.LinkedMachineID != nil && machineID == nil {
		return errors.New("le serveur cloud ne peut pas être retiré une fois lié")
	}
	return nil
}
