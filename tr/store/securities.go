package store

import (
	"context"
	"fmt"
)

// SecurityInfo содержит справочную информацию о ценной бумаге.
type SecurityInfo struct {
	SecID              string
	ShortName          string
	Name               string
	ISIN               string
	RegNumber          string
	IsTraded           uint8
	EmitentID          string
	EmitentTitle       string
	EmitentINN         string
	EmitentOKPO        string
	Type               string
	Group              string
	PrimaryBoardID     string
	MarketpriceBoardID string
}

func (s *Store) StoreSecurityInfo(ctx context.Context, infos []SecurityInfo) error {
	if len(infos) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO security_info(
	    secid, shortname, name, isin, regnumber, is_traded,
	    emitent_id, emitent_title, emitent_inn, emitent_okpo,
	    sec_type, sec_group, primary_boardid, marketprice_boardid
	)`)
	if err != nil {
		return err
	}
	for _, i := range infos {
		err = batch.Append(
			i.SecID, i.ShortName, i.Name, i.ISIN, i.RegNumber, i.IsTraded,
			i.EmitentID, i.EmitentTitle, i.EmitentINN, i.EmitentOKPO,
			i.Type, i.Group, i.PrimaryBoardID, i.MarketpriceBoardID,
		)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

// DistinctSecIDs возвращает все уникальные тикеры, встречающиеся в таблицах данных.
func (s *Store) DistinctSecIDs(ctx context.Context) ([]string, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT DISTINCT secid
		FROM (
			SELECT secid FROM super_eq
			UNION ALL
			SELECT secid FROM super_fo
			UNION ALL
			SELECT secid FROM super_fx
			UNION ALL
			SELECT ticker AS secid FROM candles
		)
		ORDER BY secid
	`)
	if err != nil {
		return nil, fmt.Errorf("distinct secids: %w", err)
	}
	defer rows.Close()

	var secids []string
	for rows.Next() {
		var secid string
		if err := rows.Scan(&secid); err != nil {
			return nil, fmt.Errorf("scan secid: %w", err)
		}
		secids = append(secids, secid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate secids: %w", err)
	}
	return secids, nil
}
