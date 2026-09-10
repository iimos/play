package store

import (
	"context"
	"time"
)

// BondDaily — дневная свеча облигации с облигационными атрибутами (ISS history).
type BondDaily struct {
	Time          time.Time
	SecID         string
	BoardID       string
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Value         float64
	Volume        uint64
	NumTrades     uint32
	AccInt        float64
	YieldClose    float64
	YieldAtWap    float64
	Waprice       float64
	Duration      *float64
	CouponPercent float64
	CouponValue   float64
	FaceValue     float64
	FaceUnit      string
	CurrencyID    string
	MatDate       *time.Time
	BondType      string
	BondSubtype   string
}

func (s *Store) StoreBondDaily(ctx context.Context, rows []BondDaily) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO bond_daily(
	    time, secid, boardid, open, high, low, close, value, volume, numtrades,
	    accint, yieldclose, yieldatwap, waprice, duration, couponpercent, couponvalue,
	    facevalue, faceunit, currencyid, matdate, bondtype, bondsubtype
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		timeStr := r.Time.Format(time.DateTime) // convert to string to eliminate timezone issues
		err = batch.Append(
			timeStr, r.SecID, r.BoardID,
			r.Open, r.High, r.Low, r.Close,
			r.Value, r.Volume, r.NumTrades,
			r.AccInt, r.YieldClose, r.YieldAtWap, r.Waprice,
			r.Duration,
			r.CouponPercent, r.CouponValue,
			r.FaceValue, r.FaceUnit, r.CurrencyID,
			r.MatDate,
			r.BondType, r.BondSubtype,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Store) GetLastBondDailyDate(ctx context.Context) (time.Time, error) {
	var lastDate time.Time
	err := s.conn.QueryRow(ctx, "SELECT max(Date(time)) FROM bond_daily").Scan(&lastDate)
	if err != nil {
		return time.Time{}, err
	}
	// ClickHouse возвращает 1970-01-01 для пустой таблицы, а не NULL.
	if lastDate.Year() < 2000 {
		return time.Time{}, nil
	}
	return lastDate, nil
}

func (s *Store) CountBondDailyForDate(ctx context.Context, date time.Time) (uint64, error) {
	dateStr := date.Format(time.DateOnly)
	var count uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM bond_daily WHERE Date(time) = ?", dateStr).Scan(&count)
	return count, err
}

func (s *Store) DeleteBondDailyPartition(ctx context.Context, date time.Time) error {
	dateStr := date.Format(time.DateOnly)
	return s.conn.Exec(ctx, "ALTER TABLE bond_daily DROP PARTITION ?", dateStr)
}
