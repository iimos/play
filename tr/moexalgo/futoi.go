package moexalgo

import (
	"fmt"
	"time"
)

// Futoi представляет открытые позиции по фьючерсным контрактам в разрезе физ. и юр. лиц.
// https://moexalgo.github.io/docs/description/futoi/
type Futoi struct {
	Time        time.Time
	Ticker      string // двухсимвольный код базового актива (или код вечного фьючерса)
	ClGroup     string // группа клиентов: FIZ / YUR
	Pos         int64  // величина открытых позиций (нетто)
	PosLong     int64  // величина длинных открытых позиций
	PosShort    int64  // величина коротких открытых позиций
	PosLongNum  int64  // количество лиц, имеющих длинную открытую позицию
	PosShortNum int64  // количество лиц, имеющих короткую открытую позицию
}

var _ FillerFrom = (*Futoi)(nil)

func (f *Futoi) IsEmpty() bool {
	if f == nil {
		return true
	}
	return f.Ticker == ""
}

func (f *Futoi) FillFrom(columns []string, data []any) error {
	var tradedate, tradetime string
	for i, cell := range data {
		if cell == nil {
			continue
		}
		switch columns[i] {
		case "tradedate":
			tradedate = cell.(string)
		case "tradetime":
			tradetime = cell.(string)
		case "ticker":
			f.Ticker = cell.(string)
		case "clgroup":
			f.ClGroup = cell.(string)
		case "pos":
			f.Pos = int64(cell.(float64))
		case "pos_long":
			f.PosLong = int64(cell.(float64))
		case "pos_short":
			f.PosShort = int64(cell.(float64))
		case "pos_long_num":
			f.PosLongNum = int64(cell.(float64))
		case "pos_short_num":
			f.PosShortNum = int64(cell.(float64))
		}
	}

	dt, err := time.Parse(time.DateTime, tradedate+" "+tradetime)
	if err != nil {
		return fmt.Errorf("failed to parse tradedate and tradetime: %q: %w", tradedate+" "+tradetime, err)
	}
	f.Time = dt
	return nil
}
