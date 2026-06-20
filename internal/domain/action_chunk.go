package domain

const MaxFirmwareParamsPerAction = 30

func ChunkExchangeParams(params []ExchangeKV, max int) [][]ExchangeKV {
	if max <= 0 || len(params) <= max {
		if len(params) == 0 {
			return nil
		}
		return [][]ExchangeKV{params}
	}
	out := make([][]ExchangeKV, 0, (len(params)+max-1)/max)
	for i := 0; i < len(params); i += max {
		end := i + max
		if end > len(params) {
			end = len(params)
		}
		out = append(out, params[i:end])
	}
	return out
}
