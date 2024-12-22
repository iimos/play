package moexalgo

import (
	"fmt"
	"time"
)

type OrderStat struct {
	Time          time.Time `json:"time" csv:"time"`
	SecID         string    `json:"secid" csv:"secid"`
	PutOrdersB    int32     `json:"put_orders_b" csv:"put_orders_b"`
	PutOrdersS    int32     `json:"put_orders_s" csv:"put_orders_s"`
	PutValB       float64   `json:"put_val_b" csv:"put_val_b"`
	PutValS       float64   `json:"put_val_s" csv:"put_val_s"`
	PutVolB       int32     `json:"put_vol_b" csv:"put_vol_b"`
	PutVolS       int32     `json:"put_vol_s" csv:"put_vol_s"`
	PutVwapB      float64   `json:"put_vwap_b" csv:"put_vwap_b"`
	PutVwapS      float64   `json:"put_vwap_s" csv:"put_vwap_s"`
	PutVol        int32     `json:"put_vol" csv:"put_vol"`
	PutVal        float64   `json:"put_val" csv:"put_val"`
	PutOrders     int32     `json:"put_orders" csv:"put_orders"`
	CancelOrdersB int32     `json:"cancel_orders_b" csv:"cancel_orders_b"`
	CancelOrdersS int32     `json:"cancel_orders_s" csv:"cancel_orders_s"`
	CancelValB    float64   `json:"cancel_val_b" csv:"cancel_val_b"`
	CancelValS    float64   `json:"cancel_val_s" csv:"cancel_val_s"`
	CancelVolB    int32     `json:"cancel_vol_b" csv:"cancel_vol_b"`
	CancelVolS    int64     `json:"cancel_vol_s" csv:"cancel_vol_s"`
	CancelVwapB   float64   `json:"cancel_vwap_b" csv:"cancel_vwap_b"`
	CancelVwapS   float64   `json:"cancel_vwap_s" csv:"cancel_vwap_s"`
	CancelVol     int64     `json:"cancel_vol" csv:"cancel_vol"`
	CancelVal     float64   `json:"cancel_val" csv:"cancel_val"`
	CancelOrders  int64     `json:"cancel_orders" csv:"cancel_orders"`
}

var _ FillerFrom = (*OrderStat)(nil)

func (o *OrderStat) IsEmpty() bool {
	if o == nil {
		return true
	}
	empty := OrderStat{Time: o.Time, SecID: o.SecID}
	return *o == empty
}

func (o *OrderStat) FillFrom(columns []string, data []any) error {
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
		case "put_orders_b":
			o.PutOrdersB = int32(number(cell))
		case "put_orders_s":
			o.PutOrdersS = int32(number(cell))
		case "put_val_b":
			o.PutValB = float64(number(cell))
		case "put_val_s":
			o.PutValS = float64(number(cell))
		case "put_vol_b":
			o.PutVolB = int32(number(cell))
		case "put_vol_s":
			o.PutVolS = int32(number(cell))
		case "put_vwap_b":
			o.PutVwapB = float64(number(cell))
		case "put_vwap_s":
			o.PutVwapS = float64(number(cell))
		case "put_vol":
			o.PutVol = int32(number(cell))
		case "put_val":
			o.PutVal = float64(number(cell))
		case "put_orders":
			o.PutOrders = int32(number(cell))
		case "cancel_orders_b":
			o.CancelOrdersB = int32(number(cell))
		case "cancel_orders_s":
			o.CancelOrdersS = int32(number(cell))
		case "cancel_val_b":
			o.CancelValB = float64(number(cell))
		case "cancel_val_s":
			o.CancelValS = float64(number(cell))
		case "cancel_vol_b":
			o.CancelVolB = int32(number(cell))
		case "cancel_vol_s":
			o.CancelVolS = int64(number(cell))
		case "cancel_vwap_b":
			o.CancelVwapB = float64(number(cell))
		case "cancel_vwap_s":
			o.CancelVwapS = float64(number(cell))
		case "cancel_vol":
			o.CancelVol = int64(number(cell))
		case "cancel_val":
			o.CancelVal = float64(number(cell))
		case "cancel_orders":
			o.CancelOrders = int64(number(cell))
		}
	}

	dt, err := time.Parse(time.DateTime, tradedate+" "+tradetime)
	if err != nil {
		return fmt.Errorf("failed to parse tradedate and tradetime: %q: %w", tradedate+" "+tradetime, err)
	}
	o.Time = dt
	return nil
}
