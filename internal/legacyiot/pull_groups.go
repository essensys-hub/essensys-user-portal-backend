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
