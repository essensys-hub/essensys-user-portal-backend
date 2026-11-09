package legacyiot

// DefaultCommandIndices — same core set as essensys-server-backend (≤30 per poll).
var DefaultCommandIndices = []int{
	613, 607, 615, 590, 349, 350, 351, 352, 363, 425, 426, 920,
	566, 567, 568, 569, 570, 571, 572,
	574, 575, 576, 577, 578,
	582, 583, 584, 585,
}

// IdentityIndices: firmware identity + Ethernet MAC (947–952). Separate serverinfos rotation chunk.
var IdentityIndices = []int{
	0, 1, 5, 6, 7, 8, 9,
	945,
	947, 948, 949, 950, 951, 952,
}

// IndexSecurityOutletsState is Securite_PriseCoupe: 1 = outlets cut, 0 = piloted (security-outlets-2026-10-011).
// Behind a gateway it is read by the gateway; without one the cloud must ask for it itself
// (essensys-user-portal-backend#63), otherwise the apps stay on "État inconnu".
const IndexSecurityOutletsState = 440

// SecurityOutletsIndices: security outlets state, a short chunk of its own in the serverinfos rotation.
var SecurityOutletsIndices = []int{IndexSecurityOutletsState}
