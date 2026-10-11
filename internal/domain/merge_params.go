package domain

import (
	"sort"
	"strconv"
)

// blockOpposite pairs each light/shutter index with the index that undoes it
// (design D2 of portal-action-merge-2026-10-007):
//   - lights: off 605+i <-> on 611+i (i = 0..5)
//   - shutters: open 617/618/619 <-> close 620/621/622, as emitted by the portal
//     (ShuttersPage.tsx). The wiki table d'échange still lists another pairing
//     that has not been checked against BP_MQX_ETH: fix it here if it wins.
var blockOpposite = func() map[int]int {
	m := make(map[int]int, IndexLightEnd-IndexLightStart+1)
	pair := func(a, b int) { m[a], m[b] = b, a }
	for i := 0; i < 6; i++ {
		pair(605+i, 611+i)
	}
	for i := 0; i < 3; i++ {
		pair(617+i, 620+i)
	}
	return m
}()

// OppositeBlockIndex returns the index that undoes k in the 605..622 block.
func OppositeBlockIndex(k int) (int, bool) {
	o, ok := blockOpposite[k]
	return o, ok
}

func isBlockIndex(k int) bool { return k >= IndexLightStart && k <= IndexLightEnd }

// IsPureBlockAction reports whether an action only carries the light/shutter
// block (590 + 605..622). Only such actions are coalesced (design D3).
func IsPureBlockAction(params []ExchangeKV) bool {
	hasBlock := false
	for _, p := range params {
		switch {
		case isBlockIndex(p.K):
			hasBlock = true
		case p.K == IndexScenario:
		default:
			return false
		}
	}
	return hasBlock
}

// MergeBlockActions replays pure light/shutter block actions in order and
// returns one complete block with the same effect (design D1):
//   - masks of different commands accumulate (bitwise OR);
//   - for the same lamp or shutter the last intent wins: setting a bit clears
//     it on the opposite index (on then off = off);
//   - "0" values added by ExpandLegacyScenarioBlock carry no intent;
//   - a non-numeric value replaces the index (most recent wins), as in the
//     legacy BitwiseFusion;
//   - 590 is never merged: the block trigger is always "1".
func MergeBlockActions(chunks ...[]ExchangeKV) []ExchangeKV {
	masks := make(map[int]int)
	raw := make(map[int]string)
	for _, chunk := range chunks {
		for _, p := range chunk {
			if !isBlockIndex(p.K) {
				continue
			}
			m, err := strconv.Atoi(p.V)
			if err != nil {
				raw[p.K] = p.V
				delete(masks, p.K)
				continue
			}
			if m == 0 {
				continue
			}
			delete(raw, p.K)
			masks[p.K] |= m
			if o, ok := blockOpposite[p.K]; ok {
				if _, isRaw := raw[o]; !isRaw {
					masks[o] &^= m
				}
			}
		}
	}

	out := make([]ExchangeKV, 0, IndexLightEnd-IndexLightStart+2)
	out = append(out, ExchangeKV{K: IndexScenario, V: "1"})
	for k := IndexLightStart; k <= IndexLightEnd; k++ {
		v, ok := raw[k]
		if !ok {
			v = strconv.Itoa(masks[k])
		}
		out = append(out, ExchangeKV{K: k, V: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].K < out[j].K })
	return out
}
