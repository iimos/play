package moexalgo

type FOTradeStat struct {
	EqTradeStat
	AssetCode string  `json:"asset_code" csv:"asset_code"`
	Im        float64 `json:"im" csv:"im"`
	OiOpen    int64   `json:"oi_open" csv:"oi_open"`
	OiHigh    int64   `json:"oi_high" csv:"oi_high"`
	OiLow     int64   `json:"oi_low" csv:"oi_low"`
	OiClose   int64   `json:"oi_close" csv:"oi_close"`
}

var _ FillerFrom = (*FOTradeStat)(nil)

func (t *FOTradeStat) IsEmpty() bool {
	if t == nil {
		return true
	}
	return t.EqTradeStat.IsEmpty()
}

func (t *FOTradeStat) FillFrom(columns []string, data []any) error {
	for i, cell := range data {
		if cell == nil {
			continue
		}
		switch columns[i] {
		case "asset_code":
			t.AssetCode = cell.(string)
		case "im":
			t.Im = number(cell)
		case "oi_open":
			t.OiOpen = int64(number(cell))
		case "oi_high":
			t.OiHigh = int64(number(cell))
		case "oi_low":
			t.OiLow = int64(number(cell))
		case "oi_close":
			t.OiClose = int64(number(cell))
		}
	}
	return t.EqTradeStat.FillFrom(columns, data)
}
