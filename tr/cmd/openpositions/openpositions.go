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

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
	"golang.org/x/sync/errgroup"
)

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	Watch       bool
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
			start = time.Now().In(tz.MSK).AddDate(0, 0, -10)
		} else {
			start = lastTableDate
		}
	}
	if end.IsZero() {
		end = time.Now().In(tz.MSK)
	}

	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, tz.MSK)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, tz.MSK)

	fmt.Printf("Loading open positions data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	assets, err := fetchAssets(ctx, moexSess)
	if err != nil {
		return err
	}

	// Автостарт взят из таблицы, а не из --start: идём от старых дней к новым.
	// Тогда обрыв загрузки оставляет последнюю дату таблицы перед дырой, и
	// повторный запуск без флагов продолжает с неё (см. watch.DateRange).
	asc := opts.StartDate.IsZero()

	// Отмену контекста (Ctrl-C) уважаем при скачивании, но не между DROP
	// PARTITION и INSERT: иначе прерывание оставит день пустым.
	wctx := context.WithoutCancel(ctx)

	var emptyDays watch.EmptyDays
	emptyDays.SkipWeekends() // по сб/вс источник обычно пуст; торговые субботы печатаются отдельно
	defer emptyDays.Report("iss_openpositions")

	for _, d := range watch.DayRange(start, end, asc, opts.StartDate.IsZero()) {
		dateStr := d.Format(time.DateOnly)
		msg, err := func() (string, error) {
			var parts []string

			isLastTableDate := !lastTableDate.IsZero() && dateStr == lastTableDate.Format(time.DateOnly)
			shouldReload := opts.ForceReload || isLastTableDate

			if !shouldReload {
				count, err := storage.CountOpenPositionsForDate(ctx, d)
				if err != nil {
					return "", fmt.Errorf("count open positions for date %s: %w", dateStr, err)
				}
				if count > 0 {
					emptyDays.Present(d)
					return fmt.Sprintf("EXISTS; %d rows", count), nil
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
			data, err := fetchOpenPositions(ctx, moexSess, d, assets)
			if err != nil {
				return "", fmt.Errorf("fetch open positions for date %s: %w", dateStr, err)
			}
			if len(data) == 0 {
				if asc {
					emptyDays.Add(d)
				}
				parts = append(parts, "NO DATA; partition untouched")
				return strings.Join(parts, "; "), nil
			}
			if shouldReload {
				if err := storage.DeleteOpenPositionsPartition(wctx, d); err != nil {
					return "", fmt.Errorf("delete open positions partition for date %s: %w", dateStr, err)
				}
			}
			if err := storage.StoreOpenPositions(wctx, data); err != nil {
				return "", fmt.Errorf("store open positions for date %s: %w", dateStr, err)
			}

			emptyDays.Present(d)
			runtime.GC()
			parts = append(parts, fmt.Sprintf("STORED %d rows", len(data)))
			return strings.Join(parts, "; "), nil
		}()
		if msg != "" {
			watch.Logf("iss_openpositions: > %s: %s\n", dateStr, msg)
		}
		if err != nil {
			return err
		}
	}

	if err := watch.CheckCoverage(ctx, func(ctx context.Context, from, to time.Time) ([]time.Time, error) {
		return storage.DatesWithData(ctx, "iss_openpositions", from, to)
	}, "iss_openpositions", end); err != nil {
		return fmt.Errorf("check iss_openpositions coverage: %w", err)
	}

	if opts.Watch {
		var tracker watch.Tracker
		return watch.RunHourly(ctx, func(ctx context.Context) error {
			now := time.Now()
			for _, d := range tracker.Days(now) {
				err := watch.ReloadDay(ctx, "watch openpositions", d,
					func(ctx context.Context, d time.Time) ([]*moexalgo.OpenPosition, error) {
						assets, err := fetchAssets(ctx, moexSess)
						if err != nil {
							return nil, err
						}
						return fetchOpenPositions(ctx, moexSess, d, assets)
					},
					storage.DeleteOpenPositionsPartition,
					storage.StoreOpenPositions,
				)
				if err != nil {
					return err
				}
			}
			tracker.Commit(now)
			return nil
		})
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
