package scenario

import "fmt"

// SlotBaseIndex returns the first exchange index for scenario slot (1–8).
func SlotBaseIndex(slot int) (int, error) {
	if slot < 1 || slot > SlotCount {
		return 0, fmt.Errorf("scenario: slot out of range: %d", slot)
	}
	return Scenario1Base + (slot-1)*ParamCount, nil
}

// SlotRange returns [start, end] inclusive for slot.
func SlotRange(slot int) (start, end int, err error) {
	base, err := SlotBaseIndex(slot)
	if err != nil {
		return 0, 0, err
	}
	return base, base + ParamCount - 1, nil
}

// SlotFromIndex returns slot number if index falls in a scenario slot block.
func SlotFromIndex(index int) (int, bool) {
	if index < Scenario1Base || index > SlotLastEnd {
		return 0, false
	}
	offset := index - Scenario1Base
	slot := offset/ParamCount + 1
	if slot < 1 || slot > SlotCount {
		return 0, false
	}
	base, _ := SlotBaseIndex(slot)
	if index < base || index >= base+ParamCount {
		return 0, false
	}
	return slot, true
}

// AbsoluteIndex returns absolute table index for offset within slot.
func AbsoluteIndex(slot, offset int) (int, error) {
	if offset < 0 || offset >= ParamCount {
		return 0, fmt.Errorf("scenario: offset out of range: %d", offset)
	}
	base, err := SlotBaseIndex(slot)
	if err != nil {
		return 0, err
	}
	return base + offset, nil
}
