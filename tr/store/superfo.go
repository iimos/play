package store

import (
	"context"
	"github.com/iimos/play/tr/moexalgo"
	"time"
)

type SuperCandleFO struct {
	Time  time.Time
	SecID string
	*moexalgo.FOTradeStat
	*moexalgo.FOObStat
}

func (s *Store) StoreSuperFO(ctx context.Context, candles []*SuperCandleFO) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_fo(
	    time, secid, asset_code,
                         
        pr_open, pr_high, pr_low, pr_close, pr_std, vol, val, trades, pr_vwap, pr_change, trades_b, trades_s, val_b, val_s, vol_b, vol_s, disb, pr_vwap_b, pr_vwap_s, im, oi_open, oi_high, oi_low, oi_close, sec_pr_open, sec_pr_high, sec_pr_low, sec_pr_close,

        mid_price, micro_price, spread_l1, spread_l2, spread_l3, spread_l5, spread_l10, spread_l20, levels_b, levels_s, vol_b_l1, vol_b_l2, vol_b_l3, vol_b_l5, vol_b_l10, vol_b_l20, vol_s_l1, vol_s_l2, vol_s_l3, vol_s_l5, vol_s_l10, vol_s_l20, vwap_b_l3, vwap_b_l5, vwap_b_l10, vwap_b_l20, vwap_s_l3, vwap_s_l5, vwap_s_l10, vwap_s_l20
)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		tr := coalesce(c.FOTradeStat)
		ob := coalesce(c.FOObStat)
		timeStr := c.Time.Format(time.DateTime) // convert to string to eliminate timezone issues

		assetCode := tr.AssetCode
		if assetCode == "" {
			assetCode = ob.AssetCode
		}

		err = batch.Append(
			timeStr, c.SecID, assetCode,

			// EqTradeStat
			tr.PrOpen, tr.PrHigh, tr.PrLow, tr.PrClose, tr.PrStd, tr.Vol, tr.Val, tr.Trades, tr.PrVwap, tr.PrChange, tr.TradesB, tr.TradesS, tr.ValB, tr.ValS, tr.VolB, tr.VolS, tr.Disb, tr.PrVwapB, tr.PrVwapS, tr.Im, tr.OiOpen, tr.OiHigh, tr.OiLow, tr.OiClose, tr.SecPrOpen, tr.SecPrHigh, tr.SecPrLow, tr.SecPrClose,

			// EqObStat
			ob.MidPrice, ob.MicroPrice, ob.SpreadL1, ob.SpreadL2, ob.SpreadL3, ob.SpreadL5, ob.SpreadL10, ob.SpreadL20, ob.LevelsB, ob.LevelsS, ob.VolBL1, ob.VolBL2, ob.VolBL3, ob.VolBL5, ob.VolBL10, ob.VolBL20, ob.VolSL1, ob.VolSL2, ob.VolSL3, ob.VolSL5, ob.VolSL10, ob.VolSL20, ob.VwapBL3, ob.VwapBL5, ob.VwapBL10, ob.VwapBL20, ob.VwapSL3, ob.VwapSL5, ob.VwapSL10, ob.VwapSL20,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) CountSuperFOCandlesForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM super_fo WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}
