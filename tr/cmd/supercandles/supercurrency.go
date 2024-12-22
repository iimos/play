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

func LoadCurrencies(ctx context.Context) error {
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

	start := must(time.Parse(time.DateOnly, "2024-01-01"))
	end := must(time.Parse(time.DateOnly, "2024-11-15"))

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		//printMemUsage()
		fmt.Printf("> %s", d.Format(time.DateOnly))

		//if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		//	fmt.Printf(": WEEKEND\n")
		//	continue
		//}

		count, err := storage.CountSuperFxCandlesForDate(ctx, d)
		if err != nil {
			panic(err)
		}

		if count > 0 {
			fmt.Printf(": EXISTS; %d supercandles\n", count)
			continue
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
