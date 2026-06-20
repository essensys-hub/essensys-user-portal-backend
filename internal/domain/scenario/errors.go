package scenario

import "errors"

var (
	// ErrSlot1ServerReserved is returned when trying to write slot 1 via definition API.
	ErrSlot1ServerReserved = errors.New("scenario: slot 1 is server-reserved")
)
