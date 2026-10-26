package domain

import "sort"

const (
	IndexScenario   = 590
	IndexLightStart = 605
	IndexLightEnd   = 622
	// IndexSecurityOutlets is Scenario_Securite of scenario 1 (offset 31 of the
	// 592..632 block): 0 = nothing, 1 = cut, 2 = restore (security-outlets-2026-10-011).
	IndexSecurityOutlets = 623
)

type ExchangeKV struct {
	K int    `json:"k"`
	V string `json:"v"`
}

// ExpandLegacyScenarioBlock ensures 590 + 605..623 for light/shutter and
// security outlet commands. A missing 623 is set to "0" so that a light command
// never replays a previous cut of the security outlets.
func ExpandLegacyScenarioBlock(params []ExchangeKV) []ExchangeKV {
	hasLightShutter := false
	for _, p := range params {
		if p.K >= IndexLightStart && p.K <= IndexSecurityOutlets {
			hasLightShutter = true
			break
		}
	}
	if !hasLightShutter {
		return params
	}

	byIndex := make(map[int]string)
	for _, p := range params {
		byIndex[p.K] = p.V
	}
	if _, ok := byIndex[IndexScenario]; !ok {
		byIndex[IndexScenario] = "1"
	}
	for i := IndexLightStart; i <= IndexSecurityOutlets; i++ {
		if _, ok := byIndex[i]; !ok {
			byIndex[i] = "0"
		}
	}

	out := make([]ExchangeKV, 0, len(byIndex))
	for k, v := range byIndex {
		out = append(out, ExchangeKV{K: k, V: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].K < out[j].K })
	return out
}
