package supercandles

import (
	"context"
	"fmt"
	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

func Load(ctx context.Context) error {
	user := os.Getenv("MOEX_USER")
	pwd := os.Getenv("MOEX_PWD")

	//moexalgo.Debug = true

	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	moexSess, err := moexalgo.NewSession(moexalgo.Params{
		Username: user,
		Password: pwd,
	})
	if err != nil {
		return err
	}

	start := must(time.Parse(time.DateOnly, "2024-04-01"))
	end := must(time.Parse(time.DateOnly, "2024-10-02"))

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		printMemUsage()
		fmt.Printf("> %s", d.Format(time.DateOnly))

		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			fmt.Printf(": WEEKEND\n")
			continue
		}

		count, err := storage.CountSuperCandlesForDate(ctx, d)
		if err != nil {
			panic(err)
		}

		if count > 0 {
			fmt.Printf(": EXISTS; %d supercandles\n", count)
			continue
		}

		data, err := fetchStats(ctx, moexSess, d)
		if err != nil {
			return err
		}
		fmt.Printf(": FETCHED %d supercandles\n", len(data))

		err = storage.StoreSuperCandles(ctx, data)
		if err != nil {
			return err
		}
		runtime.GC()
	}

	//fmt.Printf("len(data) = %d\n", len(data))
	//fmt.Printf("Time: %+v\n", data[0].Time.Format(time.DateTime))
	//fmt.Printf("SecID: %+v\n", data[0].SecID)
	//fmt.Printf("data: %+v\n", data[0].TradeStat)
	//fmt.Printf("data: %+v\n", data[0].ObStat)
	//fmt.Printf("data: %+v\n", data[0].OrderStat)

	return nil
}

type statKey struct {
	time  int64
	secid string
}

func fetchStats(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*store.SuperCandle, error) {
	mu := sync.Mutex{}
	stats := make(map[statKey]*store.SuperCandle, moexalgo.DefaultPageLimit)

	get := func(t time.Time, secid string) *store.SuperCandle {
		key := statKey{time: t.Unix(), secid: secid}
		if _, ok := stats[key]; !ok {
			stats[key] = &store.SuperCandle{
				Time:  t,
				SecID: secid,
			}
		}
		return stats[key]
	}

	dateStr := date.Format(time.DateOnly)

	gr, ctx := errgroup.WithContext(ctx)
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/eq/tradestats.json?date="+dateStr, func(d *moexalgo.TradeStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).TradeStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/eq/obstats.json?date="+dateStr, func(d *moexalgo.ObStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).ObStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/eq/orderstats.json?date="+dateStr, func(d *moexalgo.OrderStat) {
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
	slices.SortFunc(statsList, func(a, b *store.SuperCandle) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			return strings.Compare(a.SecID, b.SecID)
		}
		return cmp
	})
	return statsList, nil
}

func must[T any](x T, err error) T {
	if err != nil {
		panic(err)
	}
	return x
}

// PrintMemUsage outputs the current, total and OS memory being used. As well as the number
// of garage collection cycles completed.
func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("\tAlloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
