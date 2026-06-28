package domain

import "strings"

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
