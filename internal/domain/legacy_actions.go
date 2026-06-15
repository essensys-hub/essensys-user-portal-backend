package domain

import (
	"bytes"
	"encoding/json"
)

// LegacyActionsResponse matches essensys-server-backend GET /api/myactions (field order matters).
type LegacyActionsResponse struct {
	De67f   *LegacyAlarmCommand `json:"_de67f"`
	Actions []LegacyAction      `json:"actions"`
}

type LegacyAction struct {
	GUID   string       `json:"guid"`
	Params []ExchangeKV `json:"params"`
}

type LegacyAlarmCommand struct {
	GUID string `json:"guid"`
	OBL  string `json:"obl"`
}

func (ar LegacyActionsResponse) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	buf.WriteString(`"_de67f":`)
	if ar.De67f == nil {
		buf.WriteString("null")
	} else {
		de67fJSON, err := json.Marshal(ar.De67f)
		if err != nil {
			return nil, err
		}
		buf.Write(de67fJSON)
	}
	buf.WriteString(`,"actions":`)
	actionsJSON, err := json.Marshal(ar.Actions)
	if err != nil {
		return nil, err
	}
	buf.Write(actionsJSON)
	buf.WriteString("}")
	return buf.Bytes(), nil
}
