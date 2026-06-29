package domain

import "sort"

// MergeExchangeParams applies pending actions in order; last value wins per index k.
func MergeExchangeParams(chunks ...[]ExchangeKV) []ExchangeKV {
	byK := make(map[int]string)
	for _, chunk := range chunks {
		for _, p := range chunk {
			byK[p.K] = p.V
		}
	}
	if len(byK) == 0 {
		return nil
	}
	out := make([]ExchangeKV, 0, len(byK))
	for k, v := range byK {
		out = append(out, ExchangeKV{K: k, V: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].K < out[j].K })
	return out
}
