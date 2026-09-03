package supercandles

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
)

func LoadCurrencies(ctx context.Context, opts LoadOptions) error {
	algopackToken := os.Getenv("MOEX_ALGOPACK_TOKEN")

	//moexalgo.Debug = true

	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	moexSess, err := moexalgo.NewSession(moexalgo.Params{
		Token: algopackToken,
	})
	if err != nil {
		return err
	}

	// Get last date from table to determine if we should reload it
	lastTableDate, err := storage.GetLastSuperFxDate(ctx)
	if err != nil {
		return err
	}

	start := opts.StartDate
	end := opts.EndDate

	// Use defaults if dates not provided
	if start.IsZero() {
		if lastTableDate.IsZero() {
			// Table is empty, start from 10 days ago
			start = time.Now().AddDate(0, 0, -10)
		} else {
			// Use last date from table as start date
			start = lastTableDate
		}
	}
	if end.IsZero() {
		end = time.Now()
	}

	// Normalize dates to start of day for comparison
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	fmt.Printf("Loading data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		//printMemUsage()
		fmt.Printf("> %s", d.Format(time.DateOnly))

		// Check if we need to reload this date
		// Always reload if it's the last date in the table (might be incomplete)
		shouldReload := opts.ForceReload || (!lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly))

		if !shouldReload {
			count, err := storage.CountSuperFxCandlesForDate(ctx, d)
			if err != nil {
				panic(err)
			}

			if count > 0 {
				fmt.Printf(": EXISTS; %d supercandles\n", count)
				continue
			}
		} else {
			// Delete existing data for this date before reloading
			fmt.Printf(": FORCE RELOAD")
			if !lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly) {
				fmt.Printf(" (last date in table)")
			}
			err := storage.DeleteSuperFxPartition(ctx, d)
			if err != nil {
				fmt.Printf(" (FAILED to delete partition: %v)\n", err)
				return fmt.Errorf("failed to delete partition for date %s: %w", d.Format(time.DateOnly), err)
			} else {
				fmt.Printf(" (partition deleted)")
			}
		}

		data, err := fetchFxStats(ctx, moexSess, d)
		if err != nil {
			return err
		}
		fmt.Printf(": FETCHED %d supercandles\n", len(data))

		//fmt.Printf("len(data) = %d\n", len(data))
		//fmt.Printf("Time: %+v\n", data[0].Time.Format(time.DateTime))
		//fmt.Printf("SecID: %+v\n", data[0].SecID)
		//fmt.Printf("data[0].FOTradeStat: %+v\n", data[0].EqTradeStat)
		//fmt.Printf("data[0].FOObStat: %+v\n", data[0].FOObStat)
		//fmt.Printf("data[0].OrderStat: %+v\n", data[0].OrderStat)

		err = storage.StoreSuperFx(ctx, data)
		if err != nil {
			return err
		}
		runtime.GC()
	}
	return nil
}

func fetchFxStats(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*store.SuperCandleFx, error) {
	mu := sync.Mutex{}
	stats := make(map[statKey]*store.SuperCandleFx, moexalgo.DefaultPageLimit)

	get := func(t time.Time, secid string) *store.SuperCandleFx {
		key := statKey{time: t.Unix(), secid: secid}
		if _, ok := stats[key]; !ok {
			stats[key] = &store.SuperCandleFx{
				Time:  t,
				SecID: secid,
			}
		}
		return stats[key]
	}

	dateStr := date.Format(time.DateOnly)

	gr, ctx := errgroup.WithContext(ctx)
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/fx/tradestats.json?date="+dateStr, func(d *moexalgo.EqTradeStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).EqTradeStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/fx/obstats.json?date="+dateStr, func(d *moexalgo.FOObStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).FOObStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/fx/orderstats.json?date="+dateStr, func(d *moexalgo.OrderStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).OrderStat = d
			}
		})
		return err
	})
	err := gr.Wait()
	if err != nil {
		return nil, err
	}

	statsList := maps.Values(stats)
	slices.SortFunc(statsList, func(a, b *store.SuperCandleFx) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			return strings.Compare(a.SecID, b.SecID)
		}
		return cmp
	})
	return statsList, nil
}
