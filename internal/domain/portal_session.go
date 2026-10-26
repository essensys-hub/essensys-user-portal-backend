package domain

import "time"

// OnlineTimeout est le seuil unique « en ligne » : gateway et armoire.
const OnlineTimeout = 2 * time.Minute

// IsOnline dit si un dernier contact est assez récent ; faux si lastSeen est nul.
func IsOnline(lastSeen, now time.Time, timeout time.Duration) bool {
	if lastSeen.IsZero() {
		return false
	}
	return now.Sub(lastSeen) <= timeout
}

type PortalUserInfo struct {
	ID              int     `json:"id"`
	Email           string  `json:"email"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	Role            string  `json:"role"`
	LinkedMachineID *int    `json:"linked_machine_id"`
	LinkedGatewayID *string `json:"linked_gateway_id"`
	LinkedArmoireID *int    `json:"linked_armoire_id"`
}

type PortalGatewayInfo struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	IP       string    `json:"ip"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

type PortalArmoireInfo struct {
	ID          int       `json:"id"`
	NoSerie     string    `json:"no_serie"`
	IP          string    `json:"ip"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	GeoLocation string    `json:"geo_location,omitempty"`
	Remote      bool      `json:"remote"`
	// Online : dernier contact de l'armoire de OnlineTimeout au plus (armoire-online-sans-gateway-2026-10-012).
	// Les clients s'en servent avec ou sans gateway : une armoire sans gateway n'a pas de bloc gateway.
	Online bool `json:"online"`
}

type PortalSessionResponse struct {
	User         PortalUserInfo     `json:"user"`
	PortalAccess bool               `json:"portal_access"`
	Gateway      *PortalGatewayInfo `json:"gateway,omitempty"`
	Armoire      *PortalArmoireInfo `json:"armoire,omitempty"`
}

// NewPortalSessionResponse assemble la réponse de GET /api/portal/session.
// gateway est nil sans gateway liée ; armoire est nil sans armoire résolue ou sans accès au portail.
func NewPortalSessionResponse(user PortalUserInfo, portalAccess bool, gateway *PortalGatewayInfo, armoire *MachineDetail, now time.Time) PortalSessionResponse {
	resp := PortalSessionResponse{User: user, PortalAccess: portalAccess, Gateway: gateway}
	if portalAccess && armoire != nil {
		resp.Armoire = &PortalArmoireInfo{
			ID:          armoire.ID,
			NoSerie:     armoire.NoSerie,
			IP:          armoire.IP,
			LastSeen:    armoire.LastSeen,
			GeoLocation: armoire.GeoLocation,
			Remote:      true,
			Online:      IsOnline(armoire.LastSeen, now, OnlineTimeout),
		}
	}
	return resp
}
