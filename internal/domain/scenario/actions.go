package scenario

import (
	"fmt"
	"strconv"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

func LaunchParams(slot int) ([]domain.ExchangeKV, error) {
	if slot < 2 || slot > SlotCount {
		return nil, fmt.Errorf("scenario: launch slot must be 2–8, got %d", slot)
	}
	return []domain.ExchangeKV{{
		K: IndexTrigger,
		V: strconv.Itoa(slot),
	}}, nil
}

func RestorePresetParams(slot int) ([]domain.ExchangeKV, error) {
	if slot < 2 || slot > 6 {
		return nil, fmt.Errorf("scenario: restore preset supported for slots 2–6, got %d", slot)
	}
	efface, ok := PresetEffaceValue[slot]
	if !ok {
		return nil, fmt.Errorf("scenario: no preset efface value for slot %d", slot)
	}
	idx, err := AbsoluteIndex(slot, OffsetEfface)
	if err != nil {
		return nil, err
	}
	return []domain.ExchangeKV{{K: idx, V: efface}}, nil
}

func ValidateDefinition(slot int, params map[int]string) error {
	start, end, err := SlotRange(slot)
	if err != nil {
		return err
	}
	for k, v := range params {
		if k < start || k > end {
			return fmt.Errorf("scenario: index %d outside slot %d range [%d-%d]", k, slot, start, end)
		}
		if n, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("scenario: index %d value not numeric: %q", k, v)
		} else if n < 0 || n > 255 {
			return fmt.Errorf("scenario: index %d value out of byte range: %d", k, n)
		}
	}
	return nil
}

func WriteDefinitionChunks(slot int, params map[int]string) ([][]domain.ExchangeKV, error) {
	if err := ValidateDefinition(slot, params); err != nil {
		return nil, err
	}
	if slot == 1 {
		return nil, ErrSlot1ServerReserved
	}
	start, end, err := SlotRange(slot)
	if err != nil {
		return nil, err
	}
	kvs := make([]domain.ExchangeKV, 0, ParamCount)
	for i := start; i <= end; i++ {
		v, ok := params[i]
		if !ok {
			v = "0"
		}
		kvs = append(kvs, domain.ExchangeKV{K: i, V: v})
	}
	return domain.ChunkExchangeParams(kvs, domain.MaxFirmwareParamsPerAction), nil
}
