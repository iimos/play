package store

import (
	"context"
	"time"
)

// IndexCandle — свеча индекса MOEX (сам индекс, а не фьючерс).
type IndexCandle struct {
	IndexID  string
	Interval uint16
	Time     time.Time
	End      time.Time
	Open     float64
	Close    float64
	High     float64
	Low      float64
	Value    float64
	Volume   float64
}

func (s *Store) StoreIndexCandles(ctx context.Context, rows []IndexCandle) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO index_candles(
	    indexid, interval, time, end, open, close, high, low, value, volume
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		// строкой, чтобы исключить влияние таймзоны Go/ClickHouse
		err = batch.Append(
			r.IndexID, r.Interval,
			r.Time.Format(time.DateTime), r.End.Format(time.DateTime),
			r.Open, r.Close, r.High, r.Low, r.Value, r.Volume,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

// GetLastIndexCandleDate возвращает максимальную дату свечей по всем индексам и
// интервалам. Пустое время, если таблица пуста.
func (s *Store) GetLastIndexCandleDate(ctx context.Context) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(Date(time)) FROM index_candles")
}

// GetLastIndexCandleDateFor возвращает максимальную дату свечей одного индекса.
func (s *Store) GetLastIndexCandleDateFor(ctx context.Context, indexID string) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(Date(time)) FROM index_candles WHERE indexid = ?", indexID)
}

func (s *Store) CountIndexCandlesForDate(ctx context.Context, indexID string, date time.Time, interval uint16) (uint64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM index_candles WHERE indexid = ? AND interval = ? AND Date(time) = ?",
		indexID, interval, date.Format(time.DateOnly),
	).Scan(&count)
	return count, err
}

// DeleteIndexCandlesFor удаляет свечи одного индекса за дату по интервалу.
// Точечное удаление (не DROP PARTITION), чтобы не задеть другие индексы.
func (s *Store) DeleteIndexCandlesFor(ctx context.Context, indexID string, date time.Time, interval uint16) error {
	return s.conn.Exec(ctx,
		"ALTER TABLE index_candles DELETE WHERE indexid = ? AND interval = ? AND Date(time) = ? SETTINGS mutations_sync = 2",
		indexID, interval, date.Format(time.DateOnly),
	)
}
