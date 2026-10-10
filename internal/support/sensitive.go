package support

import (
	"regexp"
	"strings"
)

// Categories of content refused in a report (D4). The detected text itself is
// never echoed back nor logged: callers only ever see the field and category.
const (
	CategoryPassword      = "password"
	CategorySecret        = "secret"
	CategoryPostalAddress = "postal_address"
	CategoryEmail         = "email"
)

type rule struct {
	category string
	re       *regexp.Regexp
}

// Rules run on normalised text: lower case, French accents folded, so
// « Mot de passe » and « mot-de-passe » match the same pattern.
var rules = []rule{
	{CategoryPassword, regexp.MustCompile(`\b(mot[ -]de[ -]passe|mdp|password|passwd|pwd|passcode|pin|code d'?acces|code acces|code wifi|cle wifi)\s*(est|is)?\s*[:=]\s*\S+`)},
	{CategoryPassword, regexp.MustCompile(`\b(mot[ -]de[ -]passe|mdp|password|passwd)\s+(est|is|c'est)\s+\S{4,}`)},
	{CategorySecret, regexp.MustCompile(`eyj[a-z0-9_-]{8,}\.[a-z0-9_-]{8,}\.[a-z0-9_-]{8,}`)},
	{CategorySecret, regexp.MustCompile(`-----begin [a-z ]*private key-----`)},
	{CategorySecret, regexp.MustCompile(`\b(ghp|gho|ghs|ghu|ghr)_[a-z0-9]{20,}|\bgithub_pat_[a-z0-9_]{20,}|\bsk-[a-z0-9_-]{20,}|\bakia[0-9a-z]{16}\b|\bnrak-[a-z0-9]{20,}|\bnrjs-[a-z0-9]{10,}|\batatt[a-z0-9_=-]{20,}`)},
	{CategorySecret, regexp.MustCompile(`\b[0-9a-f]{32,}\b`)},
	{CategorySecret, regexp.MustCompile(`[a-z0-9+/]{40,}={0,2}`)},
	{CategoryEmail, regexp.MustCompile(`[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)},
	{CategoryPostalAddress, regexp.MustCompile(`\b\d{1,4}\s*(bis|ter)?\s*,?\s*(rue|avenue|av|bd|boulevard|chemin|allee|impasse|place|route|quai|cours|lieu[ -]dit|residence|square|chaussee|faubourg|hameau|lotissement|sentier|voie)\b\.?\s+\S+`)},
	{CategoryPostalAddress, regexp.MustCompile(`(^|[^\d.,:])(\d{5})\s+([a-z][a-z'-]{2,})`)},
}

// notCityWords are words that commonly follow a 5-digit number in a technical
// report and must not be read as « code postal + ville ».
var notCityWords = map[string]bool{
	"ms": true, "sec": true, "secondes": true, "seconds": true, "minutes": true, "octets": true, "bytes": true,
	"lux": true, "watts": true, "kwh": true, "requetes": true, "requests": true, "lignes": true, "lines": true,
	"fois": true, "times": true, "tours": true, "points": true, "erreurs": true, "errors": true, "messages": true,
	"euros": true, "eur": true, "port": true, "ports": true, "tests": true, "items": true, "entrees": true,
}

var foldAccents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "ù", "u", "û", "u", "ü", "u", "ú", "u",
	"ÿ", "y", "œ", "oe", "æ", "ae", "’", "'",
)

func normalise(s string) string {
	return foldAccents.Replace(strings.ToLower(s))
}

// DetectSensitive returns the first category of sensitive content found in s,
// or "" when s looks safe to publish.
func DetectSensitive(s string) string {
	n := normalise(s)
	for _, r := range rules {
		if r.category == CategoryPostalAddress && strings.Contains(r.re.String(), `\d{5}`) {
			for _, m := range r.re.FindAllStringSubmatch(n, -1) {
				if !notCityWords[m[3]] {
					return r.category
				}
			}
			continue
		}
		if r.re.MatchString(n) {
			return r.category
		}
	}
	return ""
}
