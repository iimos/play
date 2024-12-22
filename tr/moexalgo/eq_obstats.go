package moexalgo

import (
	"fmt"
	"time"
)

type EqObStat struct {
	Time            time.Time `json:"time" csv:"time"`
	SecID           string    `json:"secid" csv:"secid"`
	SpreadBbo       float64   `json:"spread_bbo" csv:"spread_bbo"`
	SpreadLv10      float64   `json:"spread_lv10" csv:"spread_lv10"`
	Spread1mio      float64   `json:"spread_1mio" csv:"spread_1mio"`
	LevelsB         int32     `json:"levels_b" csv:"levels_b"`
	LevelsS         int32     `json:"levels_s" csv:"levels_s"`
	VolB            int64     `json:"vol_b" csv:"vol_b"`
	VolS            int64     `json:"vol_s" csv:"vol_s"`
	ValB            int64     `json:"val_b" csv:"val_b"`
	ValS            int64     `json:"val_s" csv:"val_s"`
	ImbalanceVolBbo float64   `json:"imbalance_vol_bbo" csv:"imbalance_vol_bbo"`
	ImbalanceValBbo float64   `json:"imbalance_val_bbo" csv:"imbalance_val_bbo"`
	ImbalanceVol    float64   `json:"imbalance_vol" csv:"imbalance_vol"`
	ImbalanceVal    float64   `json:"imbalance_val" csv:"imbalance_val"`
	VwapB           float64   `json:"vwap_b" csv:"vwap_b"`
	VwapS           float64   `json:"vwap_s" csv:"vwap_s"`
	VwapB1mio       float64   `json:"vwap_b_1mio" csv:"vwap_b_1mio"`
	VwapS1mio       float64   `json:"vwap_s_1mio" csv:"vwap_s_1mio"`
}

var _ FillerFrom = (*EqObStat)(nil)

func (o *EqObStat) IsEmpty() bool {
	if o == nil {
		return true
	}
	empty := EqObStat{Time: o.Time, SecID: o.SecID}
	return *o == empty
}

func (o *EqObStat) FillFrom(columns []string, data []any) error {
	var tradedate, tradetime string
	for i, cell := range data {
		if cell == nil {
			continue
		}
		switch columns[i] {
		case "tradedate":
			tradedate = cell.(string)
		case "tradetime":
			tradetime = cell.(string)
		case "secid":
			o.SecID = cell.(string)
		case "spread_bbo":
			o.SpreadBbo = float64(number(cell))
		case "spread_lv10":
			o.SpreadLv10 = float64(number(cell))
		case "spread_1mio":
			o.Spread1mio = float64(number(cell))
		case "levels_b":
			o.LevelsB = int32(number(cell))
		case "levels_s":
			o.LevelsS = int32(number(cell))
		case "vol_b":
			o.VolB = int64(number(cell))
		case "vol_s":
			o.VolS = int64(number(cell))
		case "val_b":
			o.ValB = int64(number(cell))
		case "val_s":
			o.ValS = int64(number(cell))
		case "imbalance_vol_bbo":
			o.ImbalanceVolBbo = float64(number(cell))
		case "imbalance_val_bbo":
			o.ImbalanceValBbo = float64(number(cell))
		case "imbalance_vol":
			o.ImbalanceVol = float64(number(cell))
		case "imbalance_val":
			o.ImbalanceVal = float64(number(cell))
		case "vwap_b":
			o.VwapB = float64(number(cell))
		case "vwap_s":
			o.VwapS = float64(number(cell))
		case "vwap_b_1mio":
			o.VwapB1mio = float64(number(cell))
		case "vwap_s_1mio":
			o.VwapS1mio = float64(number(cell))

		}
	}

	dt, err := time.Parse(time.DateTime, tradedate+" "+tradetime)
	if err != nil {
		return fmt.Errorf("failed to parse tradedate and tradetime: %q: %w", tradedate+" "+tradetime, err)
	}
	o.Time = dt
	return nil
}
