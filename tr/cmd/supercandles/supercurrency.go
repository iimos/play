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

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
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
			start = time.Now().In(tz.MSK).AddDate(0, 0, -10)
		} else {
			// Use last date from table as start date
			start = lastTableDate
		}
	}
	if end.IsZero() {
		end = time.Now().In(tz.MSK)
	}

	// Normalize dates to start of day for comparison
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	fmt.Printf("Loading data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	// Автостарт взят из таблицы, а не из --start: идём от старых дней к новым.
	// Тогда обрыв загрузки оставляет последнюю дату таблицы перед дырой, и
	// повторный запуск без флагов продолжает с неё (см. watch.DateRange).
	asc := opts.StartDate.IsZero()

	for _, d := range watch.DateRange(start, end, asc) {
		//printMemUsage()
		fmt.Printf("> %s", d.Format(time.DateOnly))

		// Check if we need to reload this date
		// Always reload if it's the last date in the table (might be incomplete)
		shouldReload := opts.ForceReload || (!lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly))

		if !shouldReload {
			count, err := storage.CountSuperFxCandlesForDate(ctx, d)
			if err != nil {
				return err
			}

			if count > 0 {
				fmt.Printf(": EXISTS; %d supercandles\n", count)
				continue
			}
		} else {
			fmt.Printf(": RELOAD")
			if !lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly) {
				fmt.Printf(" (last date in table)")
			}
		}

		// Сначала качаем данные и только потом удаляем партицию: иначе сбой сети
		// или пустой ответ стирает уже загруженный день. Пустой ответ оставляет
		// партицию нетронутой — день перезальётся при следующем запуске.
		data, err := fetchFxStats(ctx, moexSess, d)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			fmt.Printf(": NO DATA; partition kept\n")
			continue
		}
		if shouldReload {
			if err := storage.DeleteSuperFxPartition(ctx, d); err != nil {
				fmt.Printf(": FAILED to delete partition: %v\n", err)
				return fmt.Errorf("failed to delete partition for date %s: %w", d.Format(time.DateOnly), err)
			}
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

	if opts.Watch {
		// Первый build сразу после первичной загрузки, не дожидаясь тика watch.
		if opts.AfterReload != nil {
			opts.AfterReload(end)
		}
		return watchFX(ctx, storage, moexSess, opts.AfterReload)
	}
	return nil
}

func fetchFxStats(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*store.SuperCandleFx, error) {
	return fetchFxStatsWithLatest(ctx, sess, date, false)
}

// fetchFxStatsWithLatest загружает пятиминутные суперсвечи по валютам. При
// latest=true запрашивается только открытая пятиминутка (latest=1).
func fetchFxStatsWithLatest(ctx context.Context, sess *moexalgo.Session, date time.Time, latest bool) ([]*store.SuperCandleFx, error) {
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
	url := "datashop/algopack/fx/%s.json?date=" + dateStr
	if latest {
		url += "&latest=1"
	}

	gr, ctx := errgroup.WithContext(ctx)
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "tradestats"), func(d *moexalgo.EqTradeStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).EqTradeStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "obstats"), func(d *moexalgo.FOObStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				get(d.Time, d.SecID).FOObStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "orderstats"), func(d *moexalgo.OrderStat) {
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
