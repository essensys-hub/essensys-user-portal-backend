package domain

import "time"

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
}

type PortalSessionResponse struct {
	User         PortalUserInfo     `json:"user"`
	PortalAccess bool               `json:"portal_access"`
	Gateway      *PortalGatewayInfo `json:"gateway,omitempty"`
	Armoire      *PortalArmoireInfo `json:"armoire,omitempty"`
}
