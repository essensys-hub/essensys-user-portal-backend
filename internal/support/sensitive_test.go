package support

import (
	"strings"
	"testing"
)

// NR: NR-backend-1 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_1_password_is_rejected(t *testing.T) {
	for _, s := range []string{
		"mon mot de passe : Soleil2026",
		"Mot de passe = azerty123",
		"password: hunter22",
		"mdp=Essensys!",
		"le code wifi : 12345678",
		"mon mot de passe est Soleil2026",
	} {
		if got := DetectSensitive(s); got != CategoryPassword {
			t.Errorf("%q: got %q, want password", s, got)
		}
	}
}

// NR: NR-backend-2 essensys-hub/essensys-feature-lifecycle#15
func Test_NR_backend_2_postal_address_is_rejected(t *testing.T) {
	for _, s := range []string{
		"12 rue des Lilas 69400 Limas",
		"j'habite au 3 bis avenue Jean Jaurès",
		"5, allée des Pins",
		"code postal 69400 Limas",
		"75011 Paris",
	} {
		if got := DetectSensitive(s); got != CategoryPostalAddress {
			t.Errorf("%q: got %q, want postal_address", s, got)
		}
	}
}

// Secret-shaped fixtures are assembled at run time so the repository holds
// no literal that a secret scanner would flag.
func TestDetectSensitive_SecretsAndEmail(t *testing.T) {
	seg := strings.Repeat("A1b2C3d4", 3)
	cases := map[string]string{
		"jeton " + "ey" + "J" + seg + ".ey" + "J" + seg + "." + seg: CategorySecret,
		"-----BEGIN RSA " + "PRIVATE KEY-----":                      CategorySecret,
		"g" + "hp_" + strings.Repeat("x9", 18):                      CategorySecret,
		"clé " + strings.Repeat("0123456789abcdef", 2):              CategorySecret,
		"écrivez-moi à jean.dupont@example.fr":                      CategoryEmail,
	}
	for s, want := range cases {
		if got := DetectSensitive(s); got != want {
			t.Errorf("%q: got %q, want %q", s, got, want)
		}
	}
}

// Ordinary technical reports must go through: false positives would push
// users away from reporting at all.
func TestDetectSensitive_LegitimateTechnicalText(t *testing.T) {
	for _, s := range []string{
		"l'écran Volets affiche 25 s puis se fige",
		"version v2.0.0 (build 1) sur iPhone 17",
		"commande k=612 v=128 envoyée deux fois",
		"après 10000 ms le portail affiche une erreur 502",
		"l'app Android plante quand je tape sur Tout éteindre dans la cuisine",
		"le mot de passe temporaire n'est pas accepté",
		"impossible de changer mon mot de passe",
		"la passerelle mon.essensys.local ne répond plus depuis 20 minutes",
		"adresse MAC b8:27:eb:12:34:56",
		"erreur 65535 ports ouverts",
	} {
		if got := DetectSensitive(s); got != "" {
			t.Errorf("%q: false positive %q", s, got)
		}
	}
}
