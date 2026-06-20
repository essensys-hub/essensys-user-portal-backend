package scenario

// Firmware reference: SC944D 099-37 BP_MQX_ETH (TableEchange.h).
const (
	IndexTrigger        = 590
	IndexLastLaunched  = 591
	Scenario1Base      = 592
	ParamCount         = 41
	SlotCount          = 8
	SlotLastBase       = 879 // Scenario8
	SlotLastEnd        = 919 // Scenario8 + ParamCount - 1
	IndexFullBlockStart = 592
	IndexFullBlockEnd   = 632
)

// Offset names within enumScenario (Scenario_NB_VALEURS).
const (
	OffsetConfirme       = 0
	OffsetAlarmeON       = 1
	OffsetAlarmeConfig   = 2
	OffsetEteindrePDVLSB = 13
	OffsetAllumerCHBLSB  = 21
	OffsetOuvrirVoletsPDV = 25
	OffsetFermerVoletsPDE = 30
	OffsetSecurite       = 31
	OffsetMachines       = 32
	OffsetChaufZJ        = 33
	OffsetCumulus        = 37
	OffsetReveilReglage  = 38
	OffsetReveilON       = 39
	OffsetEfface         = 40
)

// Preset labels for UI (slots 2–8).
var DefaultSlotLabels = map[int]string{
	1: "Réservé serveur",
	2: "Je sors",
	3: "Je pars en vacances",
	4: "Je rentre",
	5: "Je vais me coucher",
	6: "Je me lève",
	7: "Personnalisé 1",
	8: "Personnalisé 2",
}

// PresetEffaceValue maps slot to Scenario_Efface init value (firmware vd_Scenario_Init).
var PresetEffaceValue = map[int]string{
	2: "2", // Je sors
	3: "3", // Vacances
	4: "4", // Je rentre
	5: "5", // Coucher
	6: "6", // Lever
}
