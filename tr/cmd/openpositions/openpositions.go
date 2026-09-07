package openpositions

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
	"golang.org/x/sync/errgroup"
)

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
}

func Load(ctx context.Context, opts LoadOptions) error {
	algopackToken := os.Getenv("MOEX_ALGOPACK_TOKEN")

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

	lastTableDate, err := storage.GetLastOpenPositionsDate(ctx)
	if err != nil {
		return err
	}

	start := opts.StartDate
	end := opts.EndDate

	if start.IsZero() {
		if lastTableDate.IsZero() {
			start = time.Now().AddDate(0, 0, -10)
		} else {
			start = lastTableDate
		}
	}
	if end.IsZero() {
		end = time.Now()
	}

	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	fmt.Printf("Loading open positions data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	assets, err := fetchAssets(ctx, moexSess)
	if err != nil {
		return err
	}

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		fmt.Printf("> %s", d.Format(time.DateOnly))

		shouldReload := opts.ForceReload || (!lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly))

		if !shouldReload {
			count, err := storage.CountOpenPositionsForDate(ctx, d)
			if err != nil {
				panic(err)
			}
			if count > 0 {
				fmt.Printf(": EXISTS; %d rows\n", count)
				continue
			}
		} else {
			fmt.Printf(": FORCE RELOAD")
			if !lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly) {
				fmt.Printf(" (last date in table)")
			}
			if err := storage.DeleteOpenPositionsPartition(ctx, d); err != nil {
				fmt.Printf(" (FAILED to delete partition: %v)\n", err)
				return fmt.Errorf("failed to delete partition for date %s: %w", d.Format(time.DateOnly), err)
			}
			fmt.Printf(" (partition deleted)")
		}

		data, err := fetchOpenPositions(ctx, moexSess, d, assets)
		if err != nil {
			return err
		}
		fmt.Printf(": FETCHED %d rows\n", len(data))

		if err := storage.StoreOpenPositions(ctx, data); err != nil {
			return err
		}

		runtime.GC()
	}
	return nil
}

func fetchAssets(ctx context.Context, sess *moexalgo.Session) ([]*moexalgo.OpenPositionAsset, error) {
	var assets []*moexalgo.OpenPositionAsset
	err := moexalgo.Get(ctx, sess, "statistics/engines/futures/markets/forts/openpositions.json", func(d *moexalgo.OpenPositionAsset) {
		if !d.IsEmpty() {
			assets = append(assets, d)
		}
	})
	if err != nil {
		return nil, err
	}
	return assets, nil
}

func activeAssetsForDate(assets []*moexalgo.OpenPositionAsset, date time.Time) []string {
	// сравниваем по календарным датам, т.к. date в локальной таймзоне, а DateFrom/DateTill в UTC
	dateStr := date.Format(time.DateOnly)
	var res []string
	for _, a := range assets {
		if !a.DateFrom.IsZero() && dateStr < a.DateFrom.Format(time.DateOnly) {
			continue
		}
		if !a.DateTill.IsZero() && dateStr > a.DateTill.Format(time.DateOnly) {
			continue
		}
		res = append(res, a.AssetCode)
	}
	return res
}

func fetchOpenPositions(ctx context.Context, sess *moexalgo.Session, date time.Time, assets []*moexalgo.OpenPositionAsset) ([]*moexalgo.OpenPosition, error) {
	dateStr := date.Format(time.DateOnly)
	active := activeAssetsForDate(assets, date)

	var (
		mu   sync.Mutex
		rows []*moexalgo.OpenPosition
	)
	gr, ctx := errgroup.WithContext(ctx)
	gr.SetLimit(10)
	for _, a := range active {
		a := a
		gr.Go(func() error {
			url := "statistics/engines/futures/markets/forts/openpositions/" + a + ".json?from=" + dateStr + "&till=" + dateStr
			err := moexalgo.Get(ctx, sess, url, func(d *moexalgo.OpenPosition) {
				if !d.IsEmpty() {
					mu.Lock()
					rows = append(rows, d)
					mu.Unlock()
				}
			})
			if err != nil {
				return err
			}
			return nil
		})
	}
	if err := gr.Wait(); err != nil {
		return nil, err
	}

	slices.SortFunc(rows, func(a, b *moexalgo.OpenPosition) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			cmp = strings.Compare(a.Asset, b.Asset)
			if cmp == 0 {
				return strings.Compare(a.ClGroup, b.ClGroup)
			}
		}
		return cmp
	})
	return rows, nil
}
