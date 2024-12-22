package moexalgo

import (
	"fmt"
	"time"
)

type FOObStat struct {
	Time       time.Time `json:"time" csv:"time"`
	SecID      string    `json:"secid" csv:"secid"`
	AssetCode  string    `json:"asset_code" csv:"asset_code"`
	MidPrice   float64   `json:"mid_price" csv:"mid_price"`
	MicroPrice float64   `json:"micro_price" csv:"micro_price"`
	SpreadL1   float64   `json:"spread_l1" csv:"spread_l1"`
	SpreadL2   float64   `json:"spread_l2" csv:"spread_l2"`
	SpreadL3   float64   `json:"spread_l3" csv:"spread_l3"`
	SpreadL5   float64   `json:"spread_l5" csv:"spread_l5"`
	SpreadL10  float64   `json:"spread_l10" csv:"spread_l10"`
	SpreadL20  float64   `json:"spread_l20" csv:"spread_l20"`
	LevelsB    int32     `json:"levels_b" csv:"levels_b"`
	LevelsS    int32     `json:"levels_s" csv:"levels_s"`
	VolBL1     int64     `json:"vol_b_l1" csv:"vol_b_l1"`
	VolBL2     int64     `json:"vol_b_l2" csv:"vol_b_l2"`
	VolBL3     int64     `json:"vol_b_l3" csv:"vol_b_l3"`
	VolBL5     int64     `json:"vol_b_l5" csv:"vol_b_l5"`
	VolBL10    int64     `json:"vol_b_l10" csv:"vol_b_l10"`
	VolBL20    int64     `json:"vol_b_l20" csv:"vol_b_l20"`
	VolSL1     int64     `json:"vol_s_l1" csv:"vol_s_l1"`
	VolSL2     int64     `json:"vol_s_l2" csv:"vol_s_l2"`
	VolSL3     int64     `json:"vol_s_l3" csv:"vol_s_l3"`
	VolSL5     int64     `json:"vol_s_l5" csv:"vol_s_l5"`
	VolSL10    int64     `json:"vol_s_l10" csv:"vol_s_l10"`
	VolSL20    int64     `json:"vol_s_l20" csv:"vol_s_l20"`
	VwapBL3    float64   `json:"vwap_b_l3" csv:"vwap_b_l3"`
	VwapBL5    float64   `json:"vwap_b_l5" csv:"vwap_b_l5"`
	VwapBL10   float64   `json:"vwap_b_l10" csv:"vwap_b_l10"`
	VwapBL20   float64   `json:"vwap_b_l20" csv:"vwap_b_l20"`
	VwapSL3    float64   `json:"vwap_s_l3" csv:"vwap_s_l3"`
	VwapSL5    float64   `json:"vwap_s_l5" csv:"vwap_s_l5"`
	VwapSL10   float64   `json:"vwap_s_l10" csv:"vwap_s_l10"`
	VwapSL20   float64   `json:"vwap_s_l20" csv:"vwap_s_l20"`
}

var _ FillerFrom = (*FOObStat)(nil)

func (o *FOObStat) IsEmpty() bool {
	if o == nil {
		return true
	}
	empty := FOObStat{Time: o.Time, SecID: o.SecID}
	return *o == empty
}

func (o *FOObStat) FillFrom(columns []string, data []any) error {
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
		case "asset_code":
			o.AssetCode = cell.(string)
		case "mid_price":
			o.MidPrice = float64(number(cell))
		case "micro_price":
			o.MicroPrice = float64(number(cell))
		case "spread_l1":
			o.SpreadL1 = float64(number(cell))
		case "spread_l2":
			o.SpreadL2 = float64(number(cell))
		case "spread_l3":
			o.SpreadL3 = float64(number(cell))
		case "spread_l5":
			o.SpreadL5 = float64(number(cell))
		case "spread_l10":
			o.SpreadL10 = float64(number(cell))
		case "spread_l20":
			o.SpreadL20 = float64(number(cell))
		case "levels_b":
			o.LevelsB = int32(number(cell))
		case "levels_s":
			o.LevelsS = int32(number(cell))
		case "vol_b_l1":
			o.VolBL1 = int64(number(cell))
		case "vol_b_l2":
			o.VolBL2 = int64(number(cell))
		case "vol_b_l3":
			o.VolBL3 = int64(number(cell))
		case "vol_b_l5":
			o.VolBL5 = int64(number(cell))
		case "vol_b_l10":
			o.VolBL10 = int64(number(cell))
		case "vol_b_l20":
			o.VolBL20 = int64(number(cell))
		case "vol_s_l1":
			o.VolSL1 = int64(number(cell))
		case "vol_s_l2":
			o.VolSL2 = int64(number(cell))
		case "vol_s_l3":
			o.VolSL3 = int64(number(cell))
		case "vol_s_l5":
			o.VolSL5 = int64(number(cell))
		case "vol_s_l10":
			o.VolSL10 = int64(number(cell))
		case "vol_s_l20":
			o.VolSL20 = int64(number(cell))
		case "vwap_b_l3":
			o.VwapBL3 = float64(number(cell))
		case "vwap_b_l5":
			o.VwapBL5 = float64(number(cell))
		case "vwap_b_l10":
			o.VwapBL10 = float64(number(cell))
		case "vwap_b_l20":
			o.VwapBL20 = float64(number(cell))
		case "vwap_s_l3":
			o.VwapSL3 = float64(number(cell))
		case "vwap_s_l5":
			o.VwapSL5 = float64(number(cell))
		case "vwap_s_l10":
			o.VwapSL10 = float64(number(cell))
		case "vwap_s_l20":
			o.VwapSL20 = float64(number(cell))
		}
	}

	dt, err := time.Parse(time.DateTime, tradedate+" "+tradetime)
	if err != nil {
		return fmt.Errorf("failed to parse tradedate and tradetime: %q: %w", tradedate+" "+tradetime, err)
	}
	o.Time = dt
	return nil
}
