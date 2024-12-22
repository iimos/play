package store

import (
	"context"
	"github.com/iimos/play/tr/moexalgo"
	"time"
	_ "time/tzdata"
)

type SuperCandleEq struct {
	Time  time.Time
	SecID string
	*moexalgo.EqTradeStat
	*moexalgo.EqObStat
	*moexalgo.OrderStat
}

func (s *Store) StoreSuperEq(ctx context.Context, candles []*SuperCandleEq) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_eq(
	    time, secid,
                         
        pr_open, pr_high, pr_low, pr_close, pr_std, vol, val, trades, pr_vwap, pr_change, trades_b, trades_s, val_b, val_s, vol_b, vol_s, disb, pr_vwap_b, pr_vwap_s, sec_pr_open, sec_pr_high, sec_pr_low, sec_pr_close,

        spread_bbo, spread_lv10, spread_1mio, levels_b, levels_s, imbalance_vol_bbo, imbalance_val_bbo, imbalance_vol, imbalance_val, vwap_b, vwap_s, vwap_b_1mio, vwap_s_1mio,

        put_orders_b, put_orders_s, put_val_b, put_val_s, put_vol_b, put_vol_s, put_vwap_b, put_vwap_s, put_vol, put_val, put_orders, cancel_orders_b, cancel_orders_s, cancel_val_b, cancel_val_s, cancel_vol_b, cancel_vol_s, cancel_vwap_b, cancel_vwap_s, cancel_vol, cancel_val, cancel_orders
)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		tr := coalesce(c.EqTradeStat)
		ob := coalesce(c.EqObStat)
		ord := coalesce(c.OrderStat)
		timeStr := c.Time.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(
			timeStr, c.SecID,

			// EqTradeStat
			tr.PrOpen, tr.PrHigh, tr.PrLow, tr.PrClose, tr.PrStd, tr.Vol, tr.Val, tr.Trades, tr.PrVwap, tr.PrChange, tr.TradesB, tr.TradesS, tr.ValB, tr.ValS, tr.VolB, tr.VolS, tr.Disb, tr.PrVwapB, tr.PrVwapS, tr.SecPrOpen, tr.SecPrHigh, tr.SecPrLow, tr.SecPrClose,

			// EqObStat
			ob.SpreadBbo, ob.SpreadLv10, ob.Spread1mio, ob.LevelsB, ob.LevelsS, ob.ImbalanceVolBbo, ob.ImbalanceValBbo, ob.ImbalanceVol, ob.ImbalanceVal, ob.VwapB, ob.VwapS, ob.VwapB1mio, ob.VwapS1mio,

			// OrderStat
			ord.PutOrdersB, ord.PutOrdersS, ord.PutValB, ord.PutValS, ord.PutVolB, ord.PutVolS, ord.PutVwapB, ord.PutVwapS, ord.PutVol, ord.PutVal, ord.PutOrders, ord.CancelOrdersB, ord.CancelOrdersS, ord.CancelValB, ord.CancelValS, ord.CancelVolB, ord.CancelVolS, ord.CancelVwapB, ord.CancelVwapS, ord.CancelVol, ord.CancelVal, ord.CancelOrders,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) CountSuperEqCandlesForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM super_eq WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}
