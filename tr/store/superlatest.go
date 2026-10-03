package store

import (
	"context"
	"fmt"
	"time"
)

// staleRetention — сколько живут версии снимка latest-таблицы до очистки.
// Окно заметно больше периода обновления (watch опрашивает каждые ~20 с) и
// совпадает с гранулярностью суперсвечей: если тик пропущен или источник
// временно вернул меньше строк, актуальный снимок остаётся доступным (FINAL).
const staleRetention = 5 * time.Minute

// *Latest-таблицы хранят снимок текущей (открытой) пятиминутки. Версия строки —
// updated_at (DEFAULT now() на стороне ClickHouse), поэтому новая публикация
// замещает предыдущую через ReplacingMergeTree; чистый снимок читается FINAL.
// После вставки удаляются версии старше staleRetention.

func (s *Store) StoreSuperEqLatest(ctx context.Context, candles []*SuperCandleEq) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_eq_latest(time, secid, `+superEqColumns+`)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		vals := append([]any{c.Time.Format(time.DateTime), c.SecID}, c.insertValues()...)
		if err = batch.Append(vals...); err != nil {
			return err
		}
	}
	if err = batch.Send(); err != nil {
		return err
	}
	return s.deleteStale(ctx, "super_eq_latest")
}

func (s *Store) StoreSuperFOLatest(ctx context.Context, candles []*SuperCandleFO) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_fo_latest(time, secid, asset_code, `+superFOColumns+`)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		tr := coalesce(c.FOTradeStat)
		ob := coalesce(c.FOObStat)
		assetCode := tr.AssetCode
		if assetCode == "" {
			assetCode = ob.AssetCode
		}
		vals := append([]any{c.Time.Format(time.DateTime), c.SecID, assetCode}, c.insertValues()...)
		if err = batch.Append(vals...); err != nil {
			return err
		}
	}
	if err = batch.Send(); err != nil {
		return err
	}
	return s.deleteStale(ctx, "super_fo_latest")
}

func (s *Store) StoreSuperFxLatest(ctx context.Context, candles []*SuperCandleFx) error {
	if len(candles) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO super_fx_latest(time, secid, `+superFxColumns+`)`)
	if err != nil {
		return err
	}
	for _, c := range candles {
		vals := append([]any{c.Time.Format(time.DateTime), c.SecID}, c.insertValues()...)
		if err = batch.Append(vals...); err != nil {
			return err
		}
	}
	if err = batch.Send(); err != nil {
		return err
	}
	return s.deleteStale(ctx, "super_fx_latest")
}

// deleteStale удаляет из latest-таблицы версии старше staleRetention, чтобы
// снимок не накапливал устаревшие строки (в т.ч. инструменты, выпавшие из
// latest=1). Только что вставленный снимок не затрагивается: его updated_at
// близок к now().
func (s *Store) deleteStale(ctx context.Context, table string) error {
	q := fmt.Sprintf(
		"ALTER TABLE %s DELETE WHERE updated_at < now() - INTERVAL %d SECOND SETTINGS mutations_sync = 1",
		table, int(staleRetention.Seconds()),
	)
	return s.conn.Exec(ctx, q)
}
