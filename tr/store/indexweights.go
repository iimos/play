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

// LastIndexWeightsDates возвращает максимальную торговую дату весов по каждому индексу.
func (s *Store) LastIndexWeightsDates(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.conn.Query(ctx, "SELECT indexid, max(tradedate) FROM index_weights GROUP BY indexid")
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

func (s *Store) CountIndexWeightsPartition(ctx context.Context, date time.Time) (uint64, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM index_weights WHERE tradedate = ?",
		date.Format(time.DateOnly),
	).Scan(&count)
	return count, err
}

// PartitionIndexWeightIndexIDs возвращает индексы, у которых есть веса за день.
func (s *Store) PartitionIndexWeightIndexIDs(ctx context.Context, date time.Time) ([]string, error) {
	rows, err := s.conn.Query(ctx,
		"SELECT DISTINCT indexid FROM index_weights WHERE tradedate = ?",
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

// DropIndexWeightsPartition удаляет партицию дня весов индексов.
func (s *Store) DropIndexWeightsPartition(ctx context.Context, date time.Time) error {
	return s.conn.Exec(ctx, "ALTER TABLE index_weights DROP PARTITION ?", date.Format(time.DateOnly))
}
