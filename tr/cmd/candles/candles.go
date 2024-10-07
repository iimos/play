package candles

import (
	"context"
	"fmt"
	"github.com/WLM1ke/gomoex"
	"github.com/iimos/play/tr/store"
	"math/rand"
	"net/http"
	"time"
)

var tickers = map[string]tickerDesc{
	// shares
	"ROSN":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
	"LKOH":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
	"TCSG":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
	"YDEX":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares, TradingStart: time.Date(2024, 07, 24, 0, 0, 0, 0, time.UTC)},
	"SBER":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
	"TRNFP": {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
	"DATA":  {Engine: gomoex.EngineStock, Market: gomoex.MarketShares, TradingStart: time.Date(2024, 10, 01, 0, 0, 0, 0, time.UTC)},

	// indexes
	"IMOEX2": {Engine: gomoex.EngineStock, Market: gomoex.MarketIndex},

	// fonds
	"LQDT": {Engine: gomoex.EngineStock, Market: gomoex.MarketShares},
}

var skipDates = map[string]bool{
	"2024-06-12": true,
	"2024-05-09": true,
	"2024-05-01": true,
}

type tickerDesc struct {
	Engine, Market string
	TradingStart   time.Time
}

func Load(ctx context.Context) error {
	s, err := store.New()
	if err != nil {
		return err
	}
	defer s.Close()

	iss := gomoex.NewISSClient(&http.Client{Timeout: 10 * time.Second})

	start := must(time.Parse(time.DateOnly, "2024-04-01"))
	end := must(time.Parse(time.DateOnly, "2024-10-06"))

	for ticker, tdesc := range tickers {
		for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
			fmt.Printf("> %s %s", ticker, d.Format(time.DateOnly))

			if d.Before(tdesc.TradingStart) {
				fmt.Printf(": before trading start\n")
				continue
			}

			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				fmt.Printf(": WEEKEND\n")
				continue
			}

			count, err := s.CountCandlesForDate(ctx, ticker, d)
			if err != nil {
				panic(err)
			}

			if count > 0 {
				fmt.Printf(": EXISTS; %d candles\n", count)
				continue
			}

			from := d.Format(time.DateOnly)
			till := from

			if skipDates[from] {
				fmt.Printf(": HOLIDAY\n")
				continue
			}

			candles, err := iss.MarketCandles(ctx, tdesc.Engine, tdesc.Market, ticker, from, till, gomoex.IntervalMin1)
			if err != nil {
				panic(err)
			}
			fmt.Printf(": FETCHED %d candles\n", len(candles))

			err = s.StoreCandles(ctx, ticker, candles)
			if err != nil {
				panic(err)
			}

			time.Sleep(100 * time.Millisecond)
			time.Sleep(time.Duration(rand.Int63n(int64(200 * time.Millisecond)))) // jitter
		}
		time.Sleep(time.Duration(rand.Int63n(int64(time.Second))))
	}
	return nil
}

func must[T any](x T, err error) T {
	if err != nil {
		panic(err)
	}
	return x
}
