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

// LastIndexCandleDates возвращает максимальную дату свечей по каждому индексу.
func (s *Store) LastIndexCandleDates(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.conn.Query(ctx, "SELECT indexid, max(Date(time)) FROM index_candles GROUP BY indexid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := map[string]time.Time{}
	for rows.Next() {
		var (
			indexID string
			d       time.Time
		)
		if err := rows.Scan(&indexID, &d); err != nil {
			return nil, err
		}
		res[indexID] = d
	}
	return res, rows.Err()
}

func (s *Store) CountIndexCandlesPartition(ctx context.Context, date time.Time) (uint64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM index_candles WHERE Date(time) = ?",
		date.Format(time.DateOnly),
	).Scan(&count)
	return count, err
}

// PartitionIndexCandleIndexIDs возвращает индексы, у которых есть свечи за день.
func (s *Store) PartitionIndexCandleIndexIDs(ctx context.Context, date time.Time) ([]string, error) {
	rows, err := s.conn.Query(ctx,
		"SELECT DISTINCT indexid FROM index_candles WHERE Date(time) = ?",
		date.Format(time.DateOnly),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DropIndexCandlesPartition удаляет партицию дня свечей индексов.
func (s *Store) DropIndexCandlesPartition(ctx context.Context, date time.Time) error {
	return s.conn.Exec(ctx, "ALTER TABLE index_candles DROP PARTITION ?", date.Format(time.DateOnly))
}
