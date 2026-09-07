package moexalgo

import (
	"fmt"
	"time"
)

// OpenPosition — дневные открытые позиции по фьючерсам в разрезе физ./юр. лиц
// (ISS statistics openpositions, ежедневный срез).
// https://iss.moex.com/iss/statistics/engines/futures/markets/forts/openpositions/
type OpenPosition struct {
	Time              time.Time
	Asset             string // ASSETCODE базового актива (полная нотация, напр. AFKS, ASTR)
	ClGroup           string // группа клиентов: FIZ / YUR (из is_fiz: 0=YUR, 1=FIZ)
	PersonsLong       int64  // количество лиц с длинной позицией
	PersonsShort      int64  // количество лиц с короткой позицией
	OpenPositionLong  int64  // величина длинных открытых позиций
	OpenPositionShort int64  // величина коротких открытых позиций (положительная)
	OIChangeLong      int64  // изменение ОИ по лонгам
	OIChangeShort     int64  // изменение ОИ по шортам
}

var _ FillerFrom = (*OpenPosition)(nil)

func (o *OpenPosition) IsEmpty() bool {
	return o == nil || o.Asset == ""
}

func (o *OpenPosition) FillFrom(columns []string, data []any) error {
	for i, cell := range data {
		if cell == nil {
			continue
		}
		switch columns[i] {
		case "tradedate":
			t, err := time.Parse(time.DateOnly, cell.(string))
			if err != nil {
				return fmt.Errorf("failed to parse tradedate %q: %w", cell, err)
			}
			o.Time = t
		case "asset":
			o.Asset = cell.(string)
		case "is_fiz":
			if cell.(float64) == 1 {
				o.ClGroup = "FIZ"
			} else {
				o.ClGroup = "YUR"
			}
		case "persons_long":
			o.PersonsLong = int64(cell.(float64))
		case "persons_short":
			o.PersonsShort = int64(cell.(float64))
		case "open_position_long":
			o.OpenPositionLong = int64(cell.(float64))
		case "open_position_short":
			o.OpenPositionShort = int64(cell.(float64))
		case "oichange_long":
			o.OIChangeLong = int64(cell.(float64))
		case "oichange_short":
			o.OIChangeShort = int64(cell.(float64))
		}
	}
	return nil
}

// OpenPositionAsset — запись из списка доступных активов (блок assets).
type OpenPositionAsset struct {
	AssetCode string
	AssetType string
	Title     string
	DateFrom  time.Time
	DateTill  time.Time
}

var _ FillerFrom = (*OpenPositionAsset)(nil)

func (a *OpenPositionAsset) IsEmpty() bool {
	return a == nil || a.AssetCode == ""
}

func (a *OpenPositionAsset) FillFrom(columns []string, data []any) error {
	for i, cell := range data {
		if cell == nil {
			continue
		}
		switch columns[i] {
		case "asset_code":
			a.AssetCode = cell.(string)
		case "asset_type":
			a.AssetType = cell.(string)
		case "title":
			a.Title = cell.(string)
		case "date_from":
			t, err := time.Parse(time.DateOnly, cell.(string))
			if err != nil {
				return fmt.Errorf("failed to parse date_from %q: %w", cell, err)
			}
			a.DateFrom = t
		case "date_till":
			t, err := time.Parse(time.DateOnly, cell.(string))
			if err != nil {
				return fmt.Errorf("failed to parse date_till %q: %w", cell, err)
			}
			a.DateTill = t
		}
	}
	return nil
}
