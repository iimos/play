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

func LoadFutures(ctx context.Context) error {
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

	start := must(time.Parse(time.DateOnly, "2024-05-01"))
	end := must(time.Parse(time.DateOnly, "2024-11-15"))

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		//printMemUsage()
		fmt.Printf("> %s", d.Format(time.DateOnly))

		//if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		//	fmt.Printf(": WEEKEND\n")
		//	continue
		//}

		count, err := storage.CountSuperFOCandlesForDate(ctx, d)
		if err != nil {
			panic(err)
		}

		if count > 0 {
			fmt.Printf(": EXISTS; %d supercandles\n", count)
			continue
		}

		data, err := fetchFOStats(ctx, moexSess, d)
		if err != nil {
			return err
		}
		fmt.Printf(": FETCHED %d supercandles\n", len(data))

		//fmt.Printf("len(data) = %d\n", len(data))
		//fmt.Printf("Time: %+v\n", data[0].Time.Format(time.DateTime))
		//fmt.Printf("SecID: %+v\n", data[0].SecID)
		//fmt.Printf("data[0].FOTradeStat: %+v\n", data[0].FOTradeStat)
		//fmt.Printf("data[0].FOObStat: %+v\n", data[0].FOObStat)

		err = storage.StoreSuperFO(ctx, data)
		if err != nil {
			return err
		}
		runtime.GC()
	}
	return nil
}

func fetchFOStats(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*store.SuperCandleFO, error) {
	mu := sync.Mutex{}
	stats := make(map[statKey]*store.SuperCandleFO, moexalgo.DefaultPageLimit)

	get := func(t time.Time, secid string) *store.SuperCandleFO {
		key := statKey{time: t.Unix(), secid: secid}
		if _, ok := stats[key]; !ok {
			stats[key] = &store.SuperCandleFO{
				Time:  t,
				SecID: secid,
			}
		}
		return stats[key]
	}

	dateStr := date.Format(time.DateOnly)

	gr, ctx := errgroup.WithContext(ctx)
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/fo/tradestats.json?date="+dateStr, func(d *moexalgo.FOTradeStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).FOTradeStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, "datashop/algopack/fo/obstats.json?date="+dateStr, func(d *moexalgo.FOObStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).FOObStat = d
			}
		})
		return err
	})
	err := gr.Wait()
	if err != nil {
		return nil, err
	}

	statsList := maps.Values(stats)
	slices.SortFunc(statsList, func(a, b *store.SuperCandleFO) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			return strings.Compare(a.SecID, b.SecID)
		}
		return cmp
	})
	return statsList, nil
}
