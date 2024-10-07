package moexalgo

import (
	"fmt"
	"time"
)

type TradeStat struct {
	Time       time.Time `json:"time" csv:"time"`
	SecID      string    `json:"secid" csv:"secid"`
	PrOpen     float64   `json:"pr_open" csv:"pr_open"`
	PrHigh     float64   `json:"pr_high" csv:"pr_high"`
	PrLow      float64   `json:"pr_low" csv:"pr_low"`
	PrClose    float64   `json:"pr_close" csv:"pr_close"`
	PrStd      float64   `json:"pr_std" csv:"pr_std"`
	Vol        int32     `json:"vol" csv:"vol"`
	Val        float64   `json:"val" csv:"val"`
	Trades     int32     `json:"trades" csv:"trades"`
	PrVwap     float64   `json:"pr_vwap" csv:"pr_vwap"`
	PrChange   float64   `json:"pr_change" csv:"pr_change"`
	TradesB    int32     `json:"trades_b" csv:"trades_b"`
	TradesS    int32     `json:"trades_s" csv:"trades_s"`
	ValB       float64   `json:"val_b" csv:"val_b"`
	ValS       float64   `json:"val_s" csv:"val_s"`
	VolB       int64     `json:"vol_b" csv:"vol_b"`
	VolS       int64     `json:"vol_s" csv:"vol_s"`
	Disb       float64   `json:"disb" csv:"disb"`
	PrVwapB    float64   `json:"pr_vwap_b" csv:"pr_vwap_b"`
	PrVwapS    float64   `json:"pr_vwap_s" csv:"pr_vwap_s"`
	SecPrOpen  int32     `json:"sec_pr_open" csv:"sec_pr_open"`
	SecPrHigh  int32     `json:"sec_pr_high" csv:"sec_pr_high"`
	SecPrLow   int32     `json:"sec_pr_low" csv:"sec_pr_low"`
	SecPrClose int32     `json:"sec_pr_close" csv:"sec_pr_close"`
}

var _ FillerFrom = (*TradeStat)(nil)

func (t *TradeStat) IsEmpty() bool {
	if t == nil {
		return true
	}
	empty := TradeStat{Time: t.Time, SecID: t.SecID}
	return *t == empty
}

func (t *TradeStat) FillFrom(columns []string, data []any) error {
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
			t.SecID = cell.(string)
		case "pr_open":
			t.PrOpen = float64(number(cell))
		case "pr_high":
			t.PrHigh = float64(number(cell))
		case "pr_low":
			t.PrLow = float64(number(cell))
		case "pr_close":
			t.PrClose = float64(number(cell))
		case "pr_std":
			t.PrStd = float64(number(cell))
		case "vol":
			t.Vol = int32(number(cell))
		case "val":
			t.Val = float64(number(cell))
		case "trades":
			t.Trades = int32(number(cell))
		case "pr_vwap":
			t.PrVwap = float64(number(cell))
		case "pr_change":
			t.PrChange = float64(number(cell))
		case "trades_b":
			t.TradesB = int32(number(cell))
		case "trades_s":
			t.TradesS = int32(number(cell))
		case "val_b":
			t.ValB = float64(number(cell))
		case "val_s":
			t.ValS = float64(number(cell))
		case "vol_b":
			t.VolB = int64(number(cell))
		case "vol_s":
			t.VolS = int64(number(cell))
		case "disb":
			t.Disb = float64(number(cell))
		case "pr_vwap_b":
			t.PrVwapB = float64(number(cell))
		case "pr_vwap_s":
			t.PrVwapS = float64(number(cell))
		case "sec_pr_open":
			t.SecPrOpen = int32(number(cell))
		case "sec_pr_high":
			t.SecPrHigh = int32(number(cell))
		case "sec_pr_low":
			t.SecPrLow = int32(number(cell))
		case "sec_pr_close":
			t.SecPrClose = int32(number(cell))
		}
	}

	dt, err := time.Parse(time.DateTime, tradedate+" "+tradetime)
	if err != nil {
		return fmt.Errorf("failed to parse tradedate and tradetime: %q: %w", tradedate+" "+tradetime, err)
	}
	t.Time = dt
	return nil
}
