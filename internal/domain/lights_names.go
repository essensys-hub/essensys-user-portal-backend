package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Custom light names (feature noms-lumieres-2026-11-004). The 33 lights are
// identified by the ids the clients already share; the original name is the
// fallback shown when no custom name is set.

// LightNameEntry is one light of the catalogue.
type LightNameEntry struct {
	ID      string `json:"id"`
	Default string `json:"default"`
}

const (
	LightNameMaxRunes = 40
	// LightNameMaxFuture is how far ahead of the cloud clock a gateway
	// timestamp is trusted (a wrong gateway clock must not win every merge).
	LightNameMaxFuture = 5 * time.Minute
)

var lightNameCatalog = []LightNameEntry{
	{ID: "terrasse", Default: "Terrasse"},
	{ID: "entree", Default: "Entrée"},
	{ID: "escalier", Default: "Escalier"},
	{ID: "deg1", Default: "Dégagement 1"},
	{ID: "deg2", Default: "Dégagement 2"},
	{ID: "pieceserv", Default: "Pièce de service"},
	{ID: "ann1", Default: "Annexe 1"},
	{ID: "ann2", Default: "Annexe 2"},
	{ID: "salon", Default: "Salon"},
	{ID: "sam", Default: "Salle à Manger"},
	{ID: "cuisine", Default: "Cuisine"},
	{ID: "sdb1", Default: "Salle de Bain 1"},
	{ID: "sdb2", Default: "Salle de Bain 2"},
	{ID: "wc1", Default: "WC 1"},
	{ID: "wc2", Default: "WC 2"},
	{ID: "bureau", Default: "Bureau"},
	{ID: "gdchamb", Default: "Grande Chambre"},
	{ID: "ptchamb1", Default: "Petite Chambre 1"},
	{ID: "ptchamb2", Default: "Petite Chambre 2"},
	{ID: "ptchamb3", Default: "Petite Chambre 3"},
	{ID: "dressing", Default: "Dressing"},
	{ID: "isalonind", Default: "Salon (indirect 1)"},
	{ID: "isalonind2", Default: "Salon (indirect 2)"},
	{ID: "icuisine", Default: "Cuisine (plans de travail)"},
	{ID: "isdb1", Default: "Salle de Bain 1 (miroir)"},
	{ID: "isdb2", Default: "Salle de Bain 2 (miroir)"},
	{ID: "igdchamb1", Default: "Grande Chambre (chevet 1)"},
	{ID: "igdchamb2", Default: "Grande Chambre (chevet 2)"},
	{ID: "iptchamb1", Default: "Petite Chambre 1 (chevet 1)"},
	{ID: "iptchamb2", Default: "Petite Chambre 1 (chevet 2)"},
	{ID: "iptchamb22", Default: "Petite Chambre 2 (chevet)"},
	{ID: "iptchamb3", Default: "Petite Chambre 3 (chevet)"},
	{ID: "idressing", Default: "Dressing (placards)"},
}

// LightNameCatalog returns a copy of the catalogue, in display order.
func LightNameCatalog() []LightNameEntry {
	out := make([]LightNameEntry, len(lightNameCatalog))
	copy(out, lightNameCatalog)
	return out
}

func LightNameByID(id string) (LightNameEntry, bool) {
	for _, l := range lightNameCatalog {
		if l.ID == id {
			return l, true
		}
	}
	return LightNameEntry{}, false
}

var ErrLightName = errors.New("light name must be 1 to 40 characters, without control characters, '<' or '>'")

// NormalizeLightName trims the name and checks it. An empty name (after
// trimming) means "back to the original name": it returns (nil, nil).
func NormalizeLightName(raw string) (*string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(s) > LightNameMaxRunes || !utf8.ValidString(s) {
		return nil, ErrLightName
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || r == '<' || r == '>' {
			return nil, ErrLightName
		}
	}
	return &s, nil
}

// CanWriteLightNames: same rule as the light timers, only the administrator
// of the linked cabinet may rename a light.
func CanWriteLightNames(role string, linkedMachineID *int) bool {
	return CanWriteLightTimers(role, linkedMachineID)
}

// LightNameState is the stored state of one light name. A nil Name is a value
// like another (reset), so that the reset propagates.
type LightNameState struct {
	Name      *string
	Version   int
	UpdatedAt time.Time
	UpdatedBy string
}

// LightNameNewer reports whether a is strictly newer than b, comparing
// (updated_at, version, updated_by) in that order. A perfect tie is not newer.
func LightNameNewer(a, b LightNameState) bool {
	at, bt := a.UpdatedAt.UTC().Truncate(time.Millisecond), b.UpdatedAt.UTC().Truncate(time.Millisecond)
	if !at.Equal(bt) {
		return at.After(bt)
	}
	if a.Version != b.Version {
		return a.Version > b.Version
	}
	return a.UpdatedBy > b.UpdatedBy
}

// ClampLightNameTime brings back to now a timestamp that is more than
// LightNameMaxFuture ahead of now.
func ClampLightNameTime(t, now time.Time) time.Time {
	if t.After(now.Add(LightNameMaxFuture)) {
		return now
	}
	return t
}
