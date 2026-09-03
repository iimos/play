package candles

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/WLM1ke/gomoex"
	"github.com/iimos/play/tr/store"
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

func Load(ctx context.Context, opts LoadOptions) error {
	s, err := store.New()
	if err != nil {
		return err
	}
	defer s.Close()

	iss := gomoex.NewISSClient(&http.Client{Timeout: 10 * time.Second})

	start := opts.StartDate
	end := opts.EndDate

	// Use defaults if dates not provided
	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -10) // today - 10 days (start of day)
	}
	if end.IsZero() {
		end = time.Now()
	}

	// Normalize dates to start of day for comparison
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	fmt.Printf("Loading data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	// Get last date from table to determine if we should reload it
	lastTableDate, err := s.GetLastCandlesDate(ctx)
	if err != nil {
		return err
	}

	for ticker, tdesc := range tickers {
		for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
			fmt.Printf("> %s %s", ticker, d.Format(time.DateOnly))

			if d.Before(tdesc.TradingStart) {
				fmt.Printf(": before trading start\n")
				continue
			}

			//if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			//	fmt.Printf(": WEEKEND\n")
			//	continue
			//}

			// Check if we need to reload this date
			// Always reload if it's the last date in the table (might be incomplete)
			shouldReload := opts.ForceReload || (!lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly))

			if !shouldReload {
				count, err := s.CountCandlesForDate(ctx, ticker, d)
				if err != nil {
					panic(err)
				}

				if count > 0 {
					fmt.Printf(": EXISTS; %d candles\n", count)
					continue
				}
			} else {
				// Delete existing data for this ticker and date before reloading
				fmt.Printf(": FORCE RELOAD")
				if !lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly) {
					fmt.Printf(" (last date in table)")
				}
				err := s.DeleteCandlesForDate(ctx, ticker, d)
				if err != nil {
					fmt.Printf(" (FAILED to delete data: %v)\n", err)
					return fmt.Errorf("failed to delete candles for ticker %s date %s: %w", ticker, d.Format(time.DateOnly), err)
				} else {
					fmt.Printf(" (data deleted)")
				}
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
