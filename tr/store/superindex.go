package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iimos/play/tr/tz"
)

// SuperIndexRow — синтезированная 5-минутная суперсвеча индекса.
type SuperIndexRow struct {
	IndexID  string
	Time     time.Time
	RefClose float64
	PrOpen   float32
	PrHigh   float32
	PrLow    float32
	PrClose  float32
	PrStd    float32
	Vol      uint64
	Val      float64
	Trades   uint64
	PrVWAP   float32
	PrChange float32
	TradesB  uint64
	TradesS  uint64
	ValB     float64
	ValS     float64
	VolB     uint64
	VolS     uint64
	Disb     float32
	PrVWAPB  float32
	PrVWAPS  float32

	SpreadBBO       float32
	SpreadLV10      float32
	Spread1Mio      float32
	LevelsB         uint64
	LevelsS         uint64
	ImbalanceVolBBO float32
	ImbalanceValBBO float32
	ImbalanceVol    float32
	ImbalanceVal    float32
	VWAPB           float32
	VWAPS           float32
	VWAPB1Mio       float32
	VWAPS1Mio       float32

	PutOrdersB    uint64
	PutOrdersS    uint64
	PutValB       float64
	PutValS       float64
	PutVolB       uint64
	PutVolS       uint64
	PutVWAPB      float32
	PutVWAPS      float32
	PutVol        uint64
	PutVal        float64
	PutOrders     uint64
	CancelOrdersB uint64
	CancelOrdersS uint64
	CancelValB    float64
	CancelValS    float64
	CancelVolB    uint64
	CancelVolS    uint64
	CancelVWAPB   float32
	CancelVWAPS   float32
	CancelVol     uint64
	CancelVal     float64
	CancelOrders  uint64

	Constituents uint16
}

// StockBar — 5-минутная суперсвеча акции (подмножество колонок super_eq).
type StockBar struct {
	SecID   string
	Time    time.Time
	PrOpen  float32
	PrHigh  float32
	PrLow   float32
	PrClose float32
	PrStd   float32
	PrVWAP  float32

	Vol    uint32
	Val    float32
	Trades uint32

	TradesB uint32
	TradesS uint32
	ValB    float32
	ValS    float32
	VolB    uint64
	VolS    uint64

	PrVWAPB float32
	PrVWAPS float32

	SpreadBBO       float32
	SpreadLV10      float32
	Spread1Mio      float32
	LevelsB         uint32
	LevelsS         uint32
	ImbalanceVolBBO float32
	ImbalanceValBBO float32
	ImbalanceVol    float32
	ImbalanceVal    float32
	VWAPB           float32
	VWAPS           float32
	VWAPB1Mio       float32
	VWAPS1Mio       float32

	PutOrdersB uint32
	PutOrdersS uint32
	PutValB    float32
	PutValS    float32
	PutVolB    uint32
	PutVolS    uint32
	PutVWAPB   float32
	PutVWAPS   float32
	PutVol     uint32
	PutVal     float32
	PutOrders  uint32

	CancelOrdersB uint32
	CancelOrdersS uint32
	CancelValB    float32
	CancelValS    float32
	CancelVolB    uint32
	CancelVolS    uint64
	CancelVWAPB   float32
	CancelVWAPS   float32
	CancelVol     uint64
	CancelVal     float32
	CancelOrders  uint64
}

// TimeClose — точка ряда (время, значение) для валютного курса.
type TimeClose struct {
	Time  time.Time
	Close float64
}

const stockBarColumns = `secid, formatDateTime(time, '%Y-%m-%d %H:%i:%S'),
	pr_open, pr_high, pr_low, pr_close, pr_std, pr_vwap,
	vol, val, trades, trades_b, trades_s, val_b, val_s, vol_b, vol_s,
	pr_vwap_b, pr_vwap_s,
	spread_bbo, spread_lv10, spread_1mio, levels_b, levels_s,
	imbalance_vol_bbo, imbalance_val_bbo, imbalance_vol, imbalance_val,
	vwap_b, vwap_s, vwap_b_1mio, vwap_s_1mio,
	put_orders_b, put_orders_s, put_val_b, put_val_s, put_vol_b, put_vol_s,
	put_vwap_b, put_vwap_s, put_vol, put_val, put_orders,
	cancel_orders_b, cancel_orders_s, cancel_val_b, cancel_val_s,
	cancel_vol_b, cancel_vol_s, cancel_vwap_b, cancel_vwap_s,
	cancel_vol, cancel_val, cancel_orders`

func (s *Store) StoreSuperIndex(ctx context.Context, rows []SuperIndexRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_index(
	    indexid, time, ref_close,
	    pr_open, pr_high, pr_low, pr_close, pr_std, vol, val, trades, pr_vwap, pr_change,
	    trades_b, trades_s, val_b, val_s, vol_b, vol_s, disb, pr_vwap_b, pr_vwap_s,
	    spread_bbo, spread_lv10, spread_1mio, levels_b, levels_s,
	    imbalance_vol_bbo, imbalance_val_bbo, imbalance_vol, imbalance_val,
	    vwap_b, vwap_s, vwap_b_1mio, vwap_s_1mio,
	    put_orders_b, put_orders_s, put_val_b, put_val_s, put_vol_b, put_vol_s,
	    put_vwap_b, put_vwap_s, put_vol, put_val, put_orders,
	    cancel_orders_b, cancel_orders_s, cancel_val_b, cancel_val_s,
	    cancel_vol_b, cancel_vol_s, cancel_vwap_b, cancel_vwap_s,
	    cancel_vol, cancel_val, cancel_orders, constituents
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		err = batch.Append(
			r.IndexID, r.Time.Format(time.DateTime), r.RefClose,
			r.PrOpen, r.PrHigh, r.PrLow, r.PrClose, r.PrStd, r.Vol, r.Val, r.Trades, r.PrVWAP, r.PrChange,
			r.TradesB, r.TradesS, r.ValB, r.ValS, r.VolB, r.VolS, r.Disb, r.PrVWAPB, r.PrVWAPS,
			r.SpreadBBO, r.SpreadLV10, r.Spread1Mio, r.LevelsB, r.LevelsS,
			r.ImbalanceVolBBO, r.ImbalanceValBBO, r.ImbalanceVol, r.ImbalanceVal,
			r.VWAPB, r.VWAPS, r.VWAPB1Mio, r.VWAPS1Mio,
			r.PutOrdersB, r.PutOrdersS, r.PutValB, r.PutValS, r.PutVolB, r.PutVolS,
			r.PutVWAPB, r.PutVWAPS, r.PutVol, r.PutVal, r.PutOrders,
			r.CancelOrdersB, r.CancelOrdersS, r.CancelValB, r.CancelValS,
			r.CancelVolB, r.CancelVolS, r.CancelVWAPB, r.CancelVWAPS,
			r.CancelVol, r.CancelVal, r.CancelOrders, r.Constituents,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) GetLastSuperIndexDate(ctx context.Context) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(Date(time)) FROM super_index")
}

func (s *Store) GetLastSuperIndexDateFor(ctx context.Context, indexID string) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(Date(time)) FROM super_index WHERE indexid = ?", indexID)
}

func (s *Store) CountSuperIndexForDate(ctx context.Context, indexID string, date time.Time) (uint64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM super_index WHERE indexid = ? AND Date(time) = ?",
		indexID, date.Format(time.DateOnly),
	).Scan(&count)
	return count, err
}

// DeleteSuperIndexFor удаляет синтезированные свечи одного индекса за дату.
func (s *Store) DeleteSuperIndexFor(ctx context.Context, indexID string, date time.Time) error {
	return s.conn.Exec(ctx,
		"ALTER TABLE super_index DELETE WHERE indexid = ? AND Date(time) = ? SETTINGS mutations_sync = 2",
		indexID, date.Format(time.DateOnly),
	)
}

// PrevTradingDate возвращает предыдущую торговую дату индекса строго раньше before.
func (s *Store) PrevTradingDate(ctx context.Context, indexID string, before time.Time) (time.Time, error) {
	var d time.Time
	err := s.conn.QueryRow(ctx,
		"SELECT max(tradedate) FROM index_weights WHERE indexid = ? AND tradedate < ?",
		indexID, before.Format(time.DateOnly),
	).Scan(&d)
	if err != nil {
		return time.Time{}, err
	}
	if d.Year() < 2000 {
		return time.Time{}, nil
	}
	return d, nil
}

// IndexDailyClose возвращает значение индекса (interval=24) за дату.
func (s *Store) IndexDailyClose(ctx context.Context, indexID string, date time.Time) (float64, bool, error) {
	var close float64
	err := s.conn.QueryRow(ctx,
		"SELECT close FROM index_candles WHERE indexid = ? AND interval = 24 AND Date(time) = ? LIMIT 1",
		indexID, date.Format(time.DateOnly),
	).Scan(&close)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return 0, false, nil
		}
		return 0, false, err
	}
	return close, true, nil
}

// IndexIntradaySession возвращает окно торгов индекса по 10-минутным свечам:
// начало первой и конец последней свечи. ok=false, если интрадей не загружен.
// Дневные свечи для этого не годятся: у interval=24 time/end — это 00:00–23:59
// календарного дня, а не границы сессии. Глубже окна загрузки интрадея его нет —
// вызывающий использует фолбэк основной сессии (см. build-index-super).
func (s *Store) IndexIntradaySession(ctx context.Context, indexID string, date time.Time) (time.Time, time.Time, bool, error) {
	var first, last time.Time
	err := s.conn.QueryRow(ctx, `
		SELECT min(time), max(end)
		FROM index_candles
		WHERE indexid = ? AND interval = 10 AND Date(time) = ?
	`, indexID, date.Format(time.DateOnly)).Scan(&first, &last)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	if first.IsZero() || first.Year() < 2000 || last.Year() < 2000 {
		return time.Time{}, time.Time{}, false, nil
	}
	return first, last, true, nil
}

// IndexWeightsForDate возвращает веса бумаг индекса на дату.
func (s *Store) IndexWeightsForDate(ctx context.Context, indexID string, date time.Time) ([]IndexWeight, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT indexid, tradedate, ticker, shortname, secid, weight, tradingsession, trade_session_date
		FROM index_weights
		WHERE indexid = ? AND tradedate = ?
	`, indexID, date.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []IndexWeight
	for rows.Next() {
		var w IndexWeight
		if err := rows.Scan(&w.IndexID, &w.TradeDate, &w.Ticker, &w.ShortName, &w.SecID,
			&w.Weight, &w.TradingSession, &w.TradeSessionDate); err != nil {
			return nil, err
		}
		res = append(res, w)
	}
	return res, rows.Err()
}

// StockMainCloses возвращает последнюю ненулевую цену бумаг за окно [from, cutoff).
// В super_eq есть интервалы с pr_close = 0 (нет сделок), поэтому обычный argMax
// по времени вернул бы 0; берём максимум по времени среди интервалов с ценой.
// Окно шире одного дня, чтобы подхватить последнюю цену для бумаг без сделок
// в референсный день.
func (s *Store) StockMainCloses(ctx context.Context, from, cutoff time.Time) (map[string]float64, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT secid, toFloat64(argMax(pr_close, if(pr_close > 0, time, toDateTime('1970-01-01 00:00:00'))))
		FROM super_eq
		WHERE time >= ? AND time < ?
		GROUP BY secid
	`, from.Format(time.DateTime), cutoff.Format(time.DateTime))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := map[string]float64{}
	for rows.Next() {
		var secid string
		var close float64
		if err := rows.Scan(&secid, &close); err != nil {
			return nil, err
		}
		if close > 0 {
			res[secid] = close
		}
	}
	return res, rows.Err()
}

// FXMainClose возвращает последнюю ненулевую цену валюты за основную сессию.
// У курсовых инструментов pr_close = 0 в интервалах без сделок, поэтому
// обычный argMax по времени вернул бы 0; берём максимум по времени среди
// интервалов с ненулевой ценой.
func (s *Store) FXMainClose(ctx context.Context, secid string, date time.Time, cutoff time.Time) (float64, bool, error) {
	var close float64
	err := s.conn.QueryRow(ctx, `
		SELECT toFloat64(argMax(pr_close, if(pr_close > 0, time, toDateTime('1970-01-01 00:00:00'))))
		FROM super_fx
		WHERE secid = ? AND toDate(time) = ? AND time < ?
	`, secid, date.Format(time.DateOnly), cutoff.Format(time.DateTime)).Scan(&close)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return 0, false, nil
		}
		return 0, false, err
	}
	if close == 0 {
		return 0, false, nil
	}
	return close, true, nil
}

// FXCloseSeries возвращает 5-минутные цены валюты за окно [start, end).
func (s *Store) FXCloseSeries(ctx context.Context, secid string, start, end time.Time) ([]TimeClose, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT formatDateTime(time, '%Y-%m-%d %H:%i:%S'), toFloat64(pr_close)
		FROM super_fx
		WHERE secid = ? AND time >= ? AND time < ?
		ORDER BY time
	`, secid, start.Format(time.DateTime), end.Format(time.DateTime))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []TimeClose
	for rows.Next() {
		var ts string
		var close float64
		if err := rows.Scan(&ts, &close); err != nil {
			return nil, err
		}
		t, err := time.ParseInLocation(time.DateTime, ts, tz.MSK)
		if err != nil {
			return nil, fmt.Errorf("parse fx time %q: %w", ts, err)
		}
		res = append(res, TimeClose{Time: t, Close: close})
	}
	return res, rows.Err()
}

// StockBars возвращает 5-минутные суперсвечи всех бумаг за окно [start, end).
func (s *Store) StockBars(ctx context.Context, start, end time.Time) ([]StockBar, error) {
	q := "SELECT " + stockBarColumns + ` FROM super_eq WHERE time >= ? AND time < ? ORDER BY time, secid`
	rows, err := s.conn.Query(ctx, q, start.Format(time.DateTime), end.Format(time.DateTime))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []StockBar
	for rows.Next() {
		var b StockBar
		var ts string
		if err := rows.Scan(
			&b.SecID, &ts,
			&b.PrOpen, &b.PrHigh, &b.PrLow, &b.PrClose, &b.PrStd, &b.PrVWAP,
			&b.Vol, &b.Val, &b.Trades, &b.TradesB, &b.TradesS, &b.ValB, &b.ValS, &b.VolB, &b.VolS,
			&b.PrVWAPB, &b.PrVWAPS,
			&b.SpreadBBO, &b.SpreadLV10, &b.Spread1Mio, &b.LevelsB, &b.LevelsS,
			&b.ImbalanceVolBBO, &b.ImbalanceValBBO, &b.ImbalanceVol, &b.ImbalanceVal,
			&b.VWAPB, &b.VWAPS, &b.VWAPB1Mio, &b.VWAPS1Mio,
			&b.PutOrdersB, &b.PutOrdersS, &b.PutValB, &b.PutValS, &b.PutVolB, &b.PutVolS,
			&b.PutVWAPB, &b.PutVWAPS, &b.PutVol, &b.PutVal, &b.PutOrders,
			&b.CancelOrdersB, &b.CancelOrdersS, &b.CancelValB, &b.CancelValS,
			&b.CancelVolB, &b.CancelVolS, &b.CancelVWAPB, &b.CancelVWAPS,
			&b.CancelVol, &b.CancelVal, &b.CancelOrders,
		); err != nil {
			return nil, err
		}
		t, err := time.ParseInLocation(time.DateTime, ts, tz.MSK)
		if err != nil {
			return nil, fmt.Errorf("parse bar time %q: %w", ts, err)
		}
		b.Time = t
		res = append(res, b)
	}
	return res, rows.Err()
}
