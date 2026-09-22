package store

import (
	"context"
	"time"
)

// IndexWeight — вес бумаги в индексе MOEX на конкретную торговую дату.
type IndexWeight struct {
	IndexID          string
	TradeDate        time.Time
	Ticker           string
	ShortName        string
	SecID            string
	Weight           float64
	TradingSession   int32
	TradeSessionDate time.Time
}

func (s *Store) StoreIndexWeights(ctx context.Context, rows []IndexWeight) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO index_weights(
	    indexid, tradedate, ticker, shortname, secid, weight, tradingsession, trade_session_date
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		err = batch.Append(
			r.IndexID, r.TradeDate.Format(time.DateOnly), r.Ticker, r.ShortName, r.SecID,
			r.Weight, r.TradingSession, r.TradeSessionDate.Format(time.DateOnly),
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

// GetLastIndexWeightsDate возвращает максимальную торговую дату весов по всем индексам.
func (s *Store) GetLastIndexWeightsDate(ctx context.Context) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(tradedate) FROM index_weights")
}

// GetLastIndexWeightsDateFor возвращает максимальную торговую дату весов одного индекса.
func (s *Store) GetLastIndexWeightsDateFor(ctx context.Context, indexID string) (time.Time, error) {
	return s.lastDate(ctx, "SELECT max(tradedate) FROM index_weights WHERE indexid = ?", indexID)
}

func (s *Store) CountIndexWeightsForDate(ctx context.Context, indexID string, date time.Time) (uint64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM index_weights WHERE indexid = ? AND tradedate = ?",
		indexID, date.Format(time.DateOnly),
	).Scan(&count)
	return count, err
}

// DeleteIndexWeightsFor удаляет веса одного индекса за дату (точечно).
func (s *Store) DeleteIndexWeightsFor(ctx context.Context, indexID string, date time.Time) error {
	return s.conn.Exec(ctx,
		"ALTER TABLE index_weights DELETE WHERE indexid = ? AND tradedate = ? SETTINGS mutations_sync = 2",
		indexID, date.Format(time.DateOnly),
	)
}
