package domain

import (
	"errors"
	"time"
)

// Disposition des objets (feature affichage-objets-2026-11-006) : activation et
// ordre d'affichage des 33 lumières et des 16 volets et stores. Un document par
// armoire et par genre ; les identifiants sont ceux des catalogues des noms
// (#41 et #52), qui ne sont pas modifiés. Les droits, la fusion et la borne
// d'horloge sont ceux des noms.

// LayoutKind est le genre d'objets d'une disposition.
type LayoutKind string

const (
	LayoutLights   LayoutKind = "lights"
	LayoutShutters LayoutKind = "shutters"
)

var (
	ErrUnknownLight   = errors.New("unknown light id")
	ErrUnknownShutter = errors.New("unknown shutter id")
	ErrInvalidLayout  = errors.New("invalid layout")
)

// ObjectLayout est le document d'un genre. La disposition par défaut a une
// version 0 et un UpdatedAt nul : ordre du catalogue, rien de désactivé.
type ObjectLayout struct {
	Order     []string
	Disabled  []string
	Version   int
	UpdatedAt time.Time
	UpdatedBy string
}

// DefaultObjectLayout est la disposition d'une armoire qui n'a rien enregistré.
func DefaultObjectLayout() ObjectLayout {
	return ObjectLayout{Order: []string{}, Disabled: []string{}}
}

// LayoutCatalog rend une copie des identifiants du genre, dans l'ordre d'origine.
func LayoutCatalog(kind LayoutKind) []string {
	switch kind {
	case LayoutLights:
		out := make([]string, 0, len(lightNameCatalog))
		for _, l := range lightNameCatalog {
			out = append(out, l.ID)
		}
		return out
	case LayoutShutters:
		out := make([]string, 0, len(shutterNameCatalog))
		for _, s := range shutterNameCatalog {
			out = append(out, s.ID)
		}
		return out
	}
	return nil
}

// ErrUnknownObject rend l'erreur « identifiant inconnu » propre au genre.
func ErrUnknownObject(kind LayoutKind) error {
	if kind == LayoutShutters {
		return ErrUnknownShutter
	}
	return ErrUnknownLight
}

func inCatalog(kind LayoutKind) map[string]bool {
	set := map[string]bool{}
	for _, id := range LayoutCatalog(kind) {
		set[id] = true
	}
	return set
}

// ValidateLayout vérifie une écriture : identifiants du catalogue du genre
// (ErrUnknownLight ou ErrUnknownShutter), pas de doublon dans order ni dans
// disabled (ErrInvalidLayout). Un order incomplet et un disabled complet sont
// acceptés : le serveur valide, il ne complète pas.
func ValidateLayout(kind LayoutKind, order, disabled []string) error {
	known := inCatalog(kind)
	for _, list := range [][]string{order, disabled} {
		seen := map[string]bool{}
		for _, id := range list {
			if !known[id] {
				return ErrUnknownObject(kind)
			}
			if seen[id] {
				return ErrInvalidLayout
			}
			seen[id] = true
		}
	}
	return nil
}

// EffectiveLayout applique la règle de tolérance : les identifiants de order
// qui sont dans le catalogue (premier exemplaire), puis ceux du catalogue
// absents de order ; disabled est ramené au catalogue.
func EffectiveLayout(kind LayoutKind, order, disabled []string) ([]string, map[string]bool) {
	cat := LayoutCatalog(kind)
	known := inCatalog(kind)
	seen := map[string]bool{}
	ordered := make([]string, 0, len(cat))
	for _, id := range order {
		if known[id] && !seen[id] {
			seen[id] = true
			ordered = append(ordered, id)
		}
	}
	for _, id := range cat {
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}
	off := map[string]bool{}
	for _, id := range disabled {
		if known[id] {
			off[id] = true
		}
	}
	return ordered, off
}

// LayoutNewer indique si a est strictement plus récent que b, avec la règle des
// noms : (updated_at, version, updated_by). Une égalité parfaite n'est pas plus
// récente (la valeur du cloud reste).
func LayoutNewer(a, b ObjectLayout) bool {
	return LightNameNewer(
		LightNameState{Version: a.Version, UpdatedAt: a.UpdatedAt, UpdatedBy: a.UpdatedBy},
		LightNameState{Version: b.Version, UpdatedAt: b.UpdatedAt, UpdatedBy: b.UpdatedBy})
}

// CanWriteLayout : seul l'administrateur de l'armoire liée écrit une disposition.
func CanWriteLayout(role string, linkedMachineID *int) bool {
	return CanWriteLightNames(role, linkedMachineID)
}
