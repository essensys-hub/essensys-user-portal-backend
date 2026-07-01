package domain

import "time"

type AdminLoginRequest struct {
	Token string `json:"token"`
}

type AdminStatsResponse struct {
	ConnectedClients int `json:"connected_clients"`
	TotalMachines    int `json:"total_machines"`
	TotalGateways    int `json:"total_gateways"`
}

type MachineDetail struct {
	ID          int       `json:"id"`
	NoSerie     string    `json:"no_serie"`
	MacAddress  string    `json:"mac_address"`
	IP          string    `json:"ip"`
	LastSeen    time.Time `json:"last_seen"`
	RawAuth     string    `json:"raw_auth"`
	RawDecoded  string    `json:"raw_decoded"`
	GeoLocation string    `json:"geo_location"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
}

type GatewayStatus struct {
	Hostname    string                 `json:"hostname"`
	Timestamp   float64                `json:"timestamp"`
	CPU         float64                `json:"cpu_usage_percent"`
	Memory      map[string]interface{} `json:"memory"`
	Disk        map[string]interface{} `json:"disk"`
	Services    map[string]bool        `json:"services"`
	ClientCount int                    `json:"client_count"`
	IP          string                 `json:"ip"`
	LastSeen    time.Time              `json:"last_seen"`
	GeoLocation string                 `json:"geo_location"`
	Lat         float64                `json:"lat"`
	Lon         float64                `json:"lon"`
}

type Subscriber struct {
	Email      string    `json:"email"`
	DateJoined time.Time `json:"date_joined"`
}

type Newsletter struct {
	ID        string     `json:"id"`
	Subject   string     `json:"subject"`
	Content   string     `json:"content"`
	Status    string     `json:"status"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}
