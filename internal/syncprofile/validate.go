package syncprofile

import (
	"fmt"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

const MaxFirmwareIndicesPerCycle = 30

func ValidateIndexRanges(ranges []domain.IndexRange) error {
	if len(ranges) == 0 {
		return fmt.Errorf("index_ranges required")
	}
	for i, rg := range ranges {
		if rg[0] > rg[1] {
			return fmt.Errorf("index_ranges[%d]: start %d > end %d", i, rg[0], rg[1])
		}
		if rg[0] < 0 || rg[1] > 4096 {
			return fmt.Errorf("index_ranges[%d]: out of bounds", i)
		}
	}
	return nil
}

func ExpectedIndexCount(ranges []domain.IndexRange) int {
	n := 0
	for _, rg := range ranges {
		n += rg[1] - rg[0] + 1
	}
	return n
}

func PullChunkCount(ranges []domain.IndexRange) int {
	total := 0
	for _, rg := range ranges {
		count := rg[1] - rg[0] + 1
		total += (count + MaxFirmwareIndicesPerCycle - 1) / MaxFirmwareIndicesPerCycle
	}
	return total
}

func FlattenIndices(ranges []domain.IndexRange) []int {
	var out []int
	for _, rg := range ranges {
		for k := rg[0]; k <= rg[1]; k++ {
			out = append(out, k)
		}
	}
	return out
}
