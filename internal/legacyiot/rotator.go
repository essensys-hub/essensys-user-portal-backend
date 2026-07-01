package legacyiot

import "sync"

// InfoRotator cycles serverinfos index lists (default commands + identity/MAC).
type InfoRotator struct {
	mu    sync.Mutex
	slots [][]int
	slot  int
}

func NewInfoRotator() *InfoRotator {
	return &InfoRotator{
		slots: [][]int{
			DefaultCommandIndices,
			IdentityIndices,
		},
	}
}

func (r *InfoRotator) Next() []int {
	if r == nil || len(r.slots) == 0 {
		out := make([]int, len(DefaultCommandIndices))
		copy(out, DefaultCommandIndices)
		return out
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	indices := r.slots[r.slot%len(r.slots)]
	r.slot++
	out := make([]int, len(indices))
	copy(out, indices)
	return out
}
