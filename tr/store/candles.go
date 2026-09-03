package store

import (
	"context"
	"github.com/WLM1ke/gomoex"
	"time"
)

func (s *Store) StoreCandles(ctx context.Context, ticker string, candles []gomoex.Candle) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO candles(time, ticker, open, close, high, low, value, volume)")
	if err != nil {
		return err
	}
	for _, c := range candles {
		begin := c.Begin.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(begin, ticker, c.Open, c.Close, c.High, c.Low, c.Value, c.Volume)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) CountCandlesForDate(ctx context.Context, ticker string, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM candles WHERE Date(time) = ? AND ticker = ?", dateStr, ticker).Scan(&count)
	return count, err
}

func (s *Store) DeleteCandlesPartition(ctx context.Context, date time.Time) error {
	dateStr := date.Format(time.DateOnly)
	return s.conn.Exec(ctx, "ALTER TABLE candles DROP PARTITION ?", dateStr)
}

func (s *Store) GetLastCandlesDate(ctx context.Context) (time.Time, error) {
	var lastDate time.Time
	err := s.conn.QueryRow(ctx, "SELECT max(Date(time)) FROM candles").Scan(&lastDate)
	if err != nil {
		return time.Time{}, err
	}
	return lastDate, nil
}

func (s *Store) DeleteCandlesForDate(ctx context.Context, ticker string, date time.Time) error {
	dateStr := date.Format(time.DateOnly)
	return s.conn.Exec(ctx, "ALTER TABLE candles DELETE WHERE Date(time) = ? AND ticker = ?", dateStr, ticker)
}
