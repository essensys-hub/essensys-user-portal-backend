package domain

import "sort"

const (
	IndexScenario   = 590
	IndexLightStart = 605
	IndexLightEnd   = 622
)

type ExchangeKV struct {
	K int    `json:"k"`
	V string `json:"v"`
}

// ExpandLegacyScenarioBlock ensures 590 + 605..622 for light/shutter commands.
func ExpandLegacyScenarioBlock(params []ExchangeKV) []ExchangeKV {
	hasLightShutter := false
	for _, p := range params {
		if p.K >= IndexLightStart && p.K <= IndexLightEnd {
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
	for i := IndexLightStart; i <= IndexLightEnd; i++ {
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
