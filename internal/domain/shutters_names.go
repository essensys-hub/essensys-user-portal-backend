package domain

import (
	"errors"
)

// Custom shutter and blind names (feature noms-volets-2026-11-005). The 16
// shutters and blinds are identified by the ids the clients already share; the
// original name is the fallback shown when no custom name is set. Name rule,
// merge rule and clock bound are the ones of the light names (noms-lumieres).

// ShutterNameEntry is one shutter or blind of the catalogue.
type ShutterNameEntry struct {
	ID      string `json:"id"`
	Default string `json:"default"`
}

var shutterNameCatalog = []ShutterNameEntry{
	{ID: "volet1salon", Default: "Volet 1 Salon"},
	{ID: "volet2salon", Default: "Volet 2 Salon"},
	{ID: "volet3salon", Default: "Volet 3 Salon"},
	{ID: "volet1salleamanger", Default: "Volet 1 Salle à Manger"},
	{ID: "volet2salleamanger", Default: "Volet 2 Salle à Manger"},
	{ID: "volet1cuisine", Default: "Volet 1 Cuisine"},
	{ID: "volet2cuisine", Default: "Volet 2 Cuisine"},
	{ID: "voletsdb", Default: "Volet Salle de Bain 1"},
	{ID: "volet1gdchamb", Default: "Volet 1 Grande Chambre"},
	{ID: "volet2gdchamb", Default: "Volet 2 Grande Chambre"},
	{ID: "volet1ptchamb", Default: "Volet Petite Chambre 1"},
	{ID: "volet2ptchamb", Default: "Volet Petite Chambre 2"},
	{ID: "volet3ptchamb", Default: "Volet Petite Chambre 3"},
	{ID: "voletbureau", Default: "Volet Bureau"},
	{ID: "voletstore", Default: "Volet \"Store\""},
	{ID: "store", Default: "Store (banne)"},
}

// ShutterNameCatalog returns a copy of the catalogue, in display order.
func ShutterNameCatalog() []ShutterNameEntry {
	out := make([]ShutterNameEntry, len(shutterNameCatalog))
	copy(out, shutterNameCatalog)
	return out
}

func ShutterNameByID(id string) (ShutterNameEntry, bool) {
	for _, s := range shutterNameCatalog {
		if s.ID == id {
			return s, true
		}
	}
	return ShutterNameEntry{}, false
}

var ErrShutterName = errors.New("shutter name must be 1 to 40 characters, without control characters, '<' or '>'")

// NormalizeShutterName applies the light name rule. An empty name (after
// trimming) means "back to the original name": it returns (nil, nil).
func NormalizeShutterName(raw string) (*string, error) {
	n, err := NormalizeLightName(raw)
	if err != nil {
		return nil, ErrShutterName
	}
	return n, nil
}

// CanWriteShutterNames: same rule as the light names, only the administrator
// of the linked cabinet may rename a shutter.
func CanWriteShutterNames(role string, linkedMachineID *int) bool {
	return CanWriteLightNames(role, linkedMachineID)
}
