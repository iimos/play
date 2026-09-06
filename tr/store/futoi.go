package store

import (
	"context"
	"github.com/iimos/play/tr/moexalgo"
	"time"
)

func (s *Store) StoreFutoi(ctx context.Context, rows []*moexalgo.Futoi) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO futoi(
	    time, ticker, clgroup, pos, pos_long, pos_short, pos_long_num, pos_short_num
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		timeStr := r.Time.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(
			timeStr, r.Ticker, r.ClGroup, r.Pos, r.PosLong, r.PosShort, r.PosLongNum, r.PosShortNum,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) GetLastFutoiDate(ctx context.Context) (time.Time, error) {
	var lastDate time.Time
	err := s.conn.QueryRow(ctx, "SELECT max(Date(time)) FROM futoi").Scan(&lastDate)
	if err != nil {
		return time.Time{}, err
	}
	// ClickHouse возвращает 1970-01-01 для пустой таблицы, а не NULL.
	if lastDate.Year() < 2000 {
		return time.Time{}, nil
	}
	return lastDate, nil
}

func (s *Store) CountFutoiForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM futoi WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}

func (s *Store) DeleteFutoiPartition(ctx context.Context, date time.Time) error {
	dateStr := date.Format(time.DateOnly)
	return s.conn.Exec(ctx, "ALTER TABLE futoi DROP PARTITION ?", dateStr)
}
