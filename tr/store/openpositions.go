package store

import (
	"context"
	"time"

	"github.com/iimos/play/tr/moexalgo"
)

func (s *Store) StoreOpenPositions(ctx context.Context, rows []*moexalgo.OpenPosition) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO iss_openpositions(
	    time, asset, clgroup, persons_long, persons_short, open_position_long, open_position_short, oichange_long, oichange_short
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		timeStr := r.Time.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(
			timeStr, r.Asset, r.ClGroup,
			r.PersonsLong, r.PersonsShort,
			r.OpenPositionLong, r.OpenPositionShort,
			r.OIChangeLong, r.OIChangeShort,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) GetLastOpenPositionsDate(ctx context.Context) (time.Time, error) {
	var lastDate time.Time
	err := s.conn.QueryRow(ctx, "SELECT max(Date(time)) FROM iss_openpositions").Scan(&lastDate)
	if err != nil {
		return time.Time{}, err
	}
	// ClickHouse возвращает 1970-01-01 для пустой таблицы, а не NULL.
	if lastDate.Year() < 2000 {
		return time.Time{}, nil
	}
	return lastDate, nil
}

func (s *Store) CountOpenPositionsForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM iss_openpositions WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}

func (s *Store) DeleteOpenPositionsPartition(ctx context.Context, date time.Time) error {
	dateStr := date.Format(time.DateOnly)
	return s.conn.Exec(ctx, "ALTER TABLE iss_openpositions DROP PARTITION ?", dateStr)
}
