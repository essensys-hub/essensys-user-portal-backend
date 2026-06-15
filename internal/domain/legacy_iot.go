package domain

// Legacy IoT protocol types (WAN armoire clients).

type ExchangeKeyValue struct {
	K int    `json:"k"`
	V string `json:"v"`
}

type MyStatusPayload struct {
	Version string             `json:"version"`
	EK      []ExchangeKeyValue `json:"ek"`
}

type ServerInfosResponse struct {
	IsConnected bool   `json:"isconnected"`
	Infos       []int  `json:"infos"`
	NewVersion  string `json:"newversion"`
}

type LegacyMachine struct {
	HashedPkey string
	NoSerie    string
	IsActive   bool
}
