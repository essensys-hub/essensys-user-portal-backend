package scenario

// BitmaskField describes one exchange index used as a bit mask in the UI.
type BitmaskField struct {
	Index       int              `json:"index"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Bits        []BitmaskBit     `json:"bits"`
}

type BitmaskBit struct {
	Bit   int    `json:"bit"`
	Value int    `json:"value"`
	Label string `json:"label"`
}

// BitmaskMeta returns UI metadata for Scenario1 lighting/shutter indices (605–622).
func BitmaskMeta() []BitmaskField {
	return []BitmaskField{
		{
			Index: 605, Name: "Eteindre_PDV_LSB",
			Bits: []BitmaskBit{
				{0, 1, "Entrée"}, {1, 2, "Salon 1"}, {2, 4, "Salon 2"},
				{3, 8, "Dressing 1"}, {4, 16, "Dressing 2"},
			},
		},
		{
			Index: 606, Name: "Eteindre_PDV_MSB",
			Bits: []BitmaskBit{
				{5, 32, "Var. bureau"}, {6, 64, "Var. salle à manger"}, {7, 128, "Var. salon"},
			},
		},
		{
			Index: 607, Name: "Eteindre_CHB_LSB",
			Bits: []BitmaskBit{
				{0, 1, "Escalier"}, {1, 2, "Gr. chambre 1"}, {2, 4, "Gr. chambre 2"},
				{3, 8, "Pet. chambre 1-1"}, {4, 16, "Pet. chambre 1-2"},
				{5, 32, "Pet. chambre 2"}, {6, 64, "Pet. chambre 3"},
			},
		},
		{
			Index: 613, Name: "Allumer_CHB_LSB",
			Bits: []BitmaskBit{
				{0, 1, "Escalier"}, {1, 2, "Gr. chambre 1"}, {2, 4, "Gr. chambre 2"},
				{3, 8, "Pet. chambre 1-1"}, {4, 16, "Pet. chambre 1-2"},
				{5, 32, "Pet. chambre 2"}, {6, 64, "Pet. chambre 3"},
			},
		},
		{
			Index: 617, Name: "OuvrirVolets_PDV",
			Bits: []BitmaskBit{
				{0, 1, "Salon 1"}, {1, 2, "Salon 2"}, {2, 4, "Salon 3"},
				{3, 8, "SAM 1"}, {4, 16, "SAM 2"}, {5, 32, "Bureau"},
			},
		},
		{
			Index: 620, Name: "FermerVolets_PDV",
			Bits: []BitmaskBit{
				{0, 1, "Salon 1"}, {1, 2, "Salon 2"}, {2, 4, "Salon 3"},
				{3, 8, "SAM 1"}, {4, 16, "SAM 2"}, {5, 32, "Bureau"},
			},
		},
	}
}
