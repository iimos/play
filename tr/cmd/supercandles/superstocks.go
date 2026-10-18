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

func LoadStocks(ctx context.Context, opts LoadOptions) error {
	algopackToken := os.Getenv("MOEX_ALGOPACK_TOKEN")

	moexalgo.Debug = true

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
	lastTableDate, err := storage.GetLastSuperEqDate(ctx)
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

	// Отмену контекста (Ctrl-C) уважаем при скачивании, но не между DROP
	// PARTITION и INSERT: иначе прерывание оставит день пустым.
	wctx := context.WithoutCancel(ctx)

	var emptyDays watch.EmptyDays
	defer emptyDays.Report("super_eq")

	for _, d := range watch.DayRange(start, end, asc, opts.StartDate.IsZero()) {
		dateStr := d.Format(time.DateOnly)
		msg, err := func() (string, error) {
			var parts []string

			isLastTableDate := !lastTableDate.IsZero() && dateStr == lastTableDate.Format(time.DateOnly)
			shouldReload := opts.ForceReload || isLastTableDate

			if !shouldReload {
				count, err := storage.CountSuperEqCandlesForDate(ctx, d)
				if err != nil {
					return "", fmt.Errorf("count super_eq for date %s: %w", dateStr, err)
				}
				if count > 0 {
					emptyDays.Present(d)
					return fmt.Sprintf("EXISTS; %d supercandles", count), nil
				}
				parts = append(parts, "EMPTY")
			} else {
				note := "RELOAD"
				if isLastTableDate {
					note += " (last date in table)"
				}
				if opts.ForceReload && !isLastTableDate {
					note += " (--force)"
				}
				parts = append(parts, note)
			}

			// Сначала качаем данные и только потом удаляем партицию: иначе сбой
			// сети или пустой ответ стирает уже загруженный день.
			data, err := fetchEqStats(ctx, moexSess, d)
			if err != nil {
				return "", fmt.Errorf("fetch super_eq for date %s: %w", dateStr, err)
			}
			if len(data) == 0 {
				if asc {
					emptyDays.Add(d)
				}
				parts = append(parts, "NO DATA; partition untouched")
				return strings.Join(parts, "; "), nil
			}
			if shouldReload {
				if err := storage.DeleteSuperEqPartition(wctx, d); err != nil {
					return "", fmt.Errorf("delete super_eq partition for date %s: %w", dateStr, err)
				}
			}
			if err := storage.StoreSuperEq(wctx, data); err != nil {
				return "", fmt.Errorf("store super_eq for date %s: %w", dateStr, err)
			}

			emptyDays.Present(d)
			runtime.GC()
			parts = append(parts, fmt.Sprintf("STORED %d supercandles", len(data)))
			return strings.Join(parts, "; "), nil
		}()
		if msg != "" {
			watch.Logf("super_eq: > %s: %s\n", dateStr, msg)
		}
		if err != nil {
			return err
		}
	}

	if err := watch.CheckCoverage(ctx, func(ctx context.Context, from, to time.Time) ([]time.Time, error) {
		return storage.DatesWithData(ctx, "super_eq", from, to)
	}, "super_eq", end); err != nil {
		return fmt.Errorf("check super_eq coverage: %w", err)
	}

	//fmt.Printf("len(data) = %d\n", len(data))
	//fmt.Printf("Time: %+v\n", data[0].Time.Format(time.DateTime))
	//fmt.Printf("SecID: %+v\n", data[0].SecID)
	//fmt.Printf("data: %+v\n", data[0].EqTradeStat)
	//fmt.Printf("data: %+v\n", data[0].EqObStat)
	//fmt.Printf("data: %+v\n", data[0].OrderStat)

	if opts.Watch {
		// Первый build сразу после первичной загрузки, не дожидаясь тика watch.
		if opts.AfterReload != nil {
			opts.AfterReload(end)
		}
		return watchEq(ctx, storage, moexSess, opts.AfterReload)
	}

	return nil
}

func fetchEqStats(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*store.SuperCandleEq, error) {
	return fetchEqStatsWithLatest(ctx, sess, date, false)
}

// fetchEqStatsWithLatest загружает пятиминутные суперсвечи по акциям. При
// latest=true запрашивается только открытая пятиминутка (latest=1).
func fetchEqStatsWithLatest(ctx context.Context, sess *moexalgo.Session, date time.Time, latest bool) ([]*store.SuperCandleEq, error) {
	mu := sync.Mutex{}
	var epCount [3]int // строк по tradestats/obstats/orderstats
	stats := make(map[statKey]*store.SuperCandleEq, moexalgo.DefaultPageLimit)

	get := func(t time.Time, secid string) *store.SuperCandleEq {
		key := statKey{time: t.Unix(), secid: secid}
		if _, ok := stats[key]; !ok {
			stats[key] = &store.SuperCandleEq{
				Time:  t,
				SecID: secid,
			}
		}
		return stats[key]
	}

	dateStr := date.Format(time.DateOnly)
	url := "datashop/algopack/eq/%s.json?date=" + dateStr
	if latest {
		url += "&latest=1"
	}

	gr, ctx := errgroup.WithContext(ctx)
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "tradestats"), func(d *moexalgo.EqTradeStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				epCount[0]++
				get(d.Time, d.SecID).EqTradeStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "obstats"), func(d *moexalgo.EqObStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				epCount[1]++
				get(d.Time, d.SecID).EqObStat = d
			}
		})
		return err
	})
	gr.Go(func() error {
		err := moexalgo.GetAll(ctx, sess, fmt.Sprintf(url, "orderstats"), func(d *moexalgo.OrderStat) {
			if !d.IsEmpty() {
				mu.Lock()
				defer mu.Unlock()
				epCount[2]++
				get(d.Time, d.SecID).OrderStat = d
			}
		})
		return err
	})
	if err := gr.Wait(); err != nil {
		return nil, err
	}
	if err := checkPartial(date, latest, epCount[:], "tradestats", "obstats", "orderstats"); err != nil {
		return nil, err
	}

	statsList := maps.Values(stats)
	slices.SortFunc(statsList, func(a, b *store.SuperCandleEq) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			return strings.Compare(a.SecID, b.SecID)
		}
		return cmp
	})
	return statsList, nil
}
