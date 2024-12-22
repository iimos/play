package store

import (
	"context"
	"github.com/iimos/play/tr/moexalgo"
	"time"
)

type SuperCandleFx struct {
	Time  time.Time
	SecID string
	*moexalgo.EqTradeStat
	*moexalgo.FOObStat
	*moexalgo.OrderStat
}

func (s *Store) StoreSuperFx(ctx context.Context, candles []*SuperCandleFx) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_fx(
	    time, secid,

		pr_open, pr_high, pr_low, pr_close, pr_std, vol, val, trades, pr_vwap, pr_change, trades_b, trades_s, val_b, val_s, vol_b, vol_s, disb, pr_vwap_b, pr_vwap_s, sec_pr_open, sec_pr_high, sec_pr_low, sec_pr_close,

		mid_price, micro_price, spread_l1, spread_l2, spread_l3, spread_l5, spread_l10, levels_b, levels_s, vol_b_l1, vol_b_l2, vol_b_l3, vol_b_l5, vol_b_l10, vol_s_l1, vol_s_l2, vol_s_l3, vol_s_l5, vol_s_l10, vwap_b_l3, vwap_b_l5, vwap_b_l10, vwap_s_l3, vwap_s_l5, vwap_s_l10, 

		put_orders_b, put_orders_s, put_val_b, put_val_s, put_vol_b, put_vol_s, put_vwap_b, put_vwap_s, cancel_orders_b, cancel_orders_s, cancel_val_b, cancel_val_s, cancel_vol_b, cancel_vol_s, cancel_vwap_b, cancel_vwap_s                 
)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		tr := coalesce(c.EqTradeStat)
		ob := coalesce(c.FOObStat)
		ord := coalesce(c.OrderStat)
		timeStr := c.Time.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(
			timeStr, c.SecID,

			// EqTradeStat
			tr.PrOpen, tr.PrHigh, tr.PrLow, tr.PrClose, tr.PrStd, tr.Vol, tr.Val, tr.Trades, tr.PrVwap, tr.PrChange, tr.TradesB, tr.TradesS, tr.ValB, tr.ValS, tr.VolB, tr.VolS, tr.Disb, tr.PrVwapB, tr.PrVwapS, tr.SecPrOpen, tr.SecPrHigh, tr.SecPrLow, tr.SecPrClose,

			// EqObStat
			ob.MidPrice, ob.MicroPrice, ob.SpreadL1, ob.SpreadL2, ob.SpreadL3, ob.SpreadL5, ob.SpreadL10, ob.LevelsB, ob.LevelsS, ob.VolBL1, ob.VolBL2, ob.VolBL3, ob.VolBL5, ob.VolBL10, ob.VolSL1, ob.VolSL2, ob.VolSL3, ob.VolSL5, ob.VolSL10, ob.VwapBL3, ob.VwapBL5, ob.VwapBL10, ob.VwapSL3, ob.VwapSL5, ob.VwapSL10,

			// OrderStat
			ord.PutOrdersB, ord.PutOrdersS, ord.PutValB, ord.PutValS, ord.PutVolB, ord.PutVolS, ord.PutVwapB, ord.PutVwapS, ord.CancelOrdersB, ord.CancelOrdersS, ord.CancelValB, ord.CancelValS, ord.CancelVolB, ord.CancelVolS, ord.CancelVwapB, ord.CancelVwapS,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) CountSuperFxCandlesForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM super_fx WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}
