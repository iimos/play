// Package indexdata загружает данные по индексам MOEX (сам индекс, не фьючерс):
// свечи индекса (OHLC + оборот) и веса бумаг в индексе.
package indexdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/httpjson"
	"github.com/iimos/play/tr/moexindex"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"
)

const (
	candlesURLFmt   = "https://iss.moex.com/iss/engines/stock/markets/index/securities/%s/candles.json"
	analyticsURLFmt = "https://iss.moex.com/iss/statistics/engines/stock/markets/index/analytics/%s.json"

	// candlesPageSize — размер страницы эндпоинта свечей ISS.
	candlesPageSize = 500
	// weightsPageSize — размер страницы эндпоинта весов ISS.
	weightsPageSize = 100

	// intradayInterval — интрадей-интервал свечей индекса (минут).
	intradayInterval = 10
	// intradayDays — за сколько последних дней грузить интрадей-свечи.
	intradayDays = 60
)

const (
	// indexWorkers — параллелизм запросов к ISS по индексам.
	indexWorkers = 8
	// catchupDays — насколько «свежей» должна быть последняя дата индекса,
	// чтобы учитывать её при выборе старта по умолчанию (отсекает истёкшие).
	catchupDays = 30
)

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	Watch       bool
	Indices     []string
}

func Load(ctx context.Context, opts LoadOptions) error {
	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	// Пусто -> основные индексы, "all" -> все доступные (см. moexindex.Resolve).
	indices, err := moexindex.Resolve(ctx, client, opts.Indices)
	if err != nil {
		return fmt.Errorf("resolve indices: %w", err)
	}

	// Последняя загруженная дата по каждому индексу: её всегда перезаливаем.
	lastCandle := map[string]time.Time{}
	lastWeight := map[string]time.Time{}
	for _, idx := range indices {
		if t, err := storage.GetLastIndexCandleDateFor(ctx, idx); err != nil {
			return err
		} else {
			lastCandle[idx] = t
		}
		if t, err := storage.GetLastIndexWeightsDateFor(ctx, idx); err != nil {
			return err
		} else {
			lastWeight[idx] = t
		}
	}

	start := opts.StartDate
	end := opts.EndDate
	if end.IsZero() {
		end = time.Now().In(tz.MSK)
	}
	if start.IsZero() {
		// По умолчанию — с самой ранней последней даты среди свежих индексов,
		// чтобы догрузить отстающие; если данных нет — последние 10 дней.
		start = end.AddDate(0, 0, -10)
		if minT, ok := minLastDate(lastCandle, indices, end.AddDate(0, 0, -catchupDays)); ok {
			start = minT
		}
	}

	start = dayMSK(start)
	end = dayMSK(end)

	fmt.Printf("Loading index data (%v) from %s to %s\n",
		indices, start.Format(time.DateOnly), end.Format(time.DateOnly))

	intradayFrom := end.AddDate(0, 0, -intradayDays)

	for d := end; !d.Before(start); d = d.AddDate(0, 0, -1) {
		withIntraday := !d.Before(intradayFrom)

		candles, err := loadDayCandles(ctx, storage, client, indices, d, withIntraday, opts.ForceReload, lastCandle)
		if err != nil {
			return err
		}
		weights, err := loadDayWeights(ctx, storage, client, indices, d, opts.ForceReload, lastWeight)
		if err != nil {
			return err
		}
		fmt.Printf("> %s: candles %s, weights %s\n", d.Format(time.DateOnly), candles, weights)

		runtime.GC()
	}

	if opts.Watch {
		return watchIndexData(ctx, storage, client, indices)
	}
	return nil
}

// loadDayCandles догружает свечи индексов за дату. Для каждого индекса и
// интервала удаляются только его собственные строки (не весь партишен), чтобы
// не задеть остальные индексы. Индексы обрабатываются параллельно.
func loadDayCandles(
	ctx context.Context,
	storage *store.Store,
	client *http.Client,
	indices []string,
	d time.Time,
	withIntraday, force bool,
	lastByIndex map[string]time.Time,
) (int, error) {
	intervals := []uint16{24}
	if withIntraday {
		intervals = append(intervals, intradayInterval)
	}

	var (
		mu    sync.Mutex
		total int
	)
	gr, ctx := errgroup.WithContext(ctx)
	gr.SetLimit(indexWorkers)
	for _, indexID := range indices {
		indexID := indexID
		gr.Go(func() error {
			n, err := loadIndexCandles(ctx, storage, client, indexID, d, intervals, force, lastByIndex[indexID])
			if err != nil {
				return err
			}
			mu.Lock()
			total += n
			mu.Unlock()
			return nil
		})
	}
	if err := gr.Wait(); err != nil {
		return total, err
	}
	return total, nil
}

func loadIndexCandles(
	ctx context.Context,
	storage *store.Store,
	client *http.Client,
	indexID string,
	d time.Time,
	intervals []uint16,
	force bool,
	lastDate time.Time,
) (int, error) {
	total := 0
	for _, iv := range intervals {
		reload := force || sameDay(lastDate, d)
		if !reload {
			count, err := storage.CountIndexCandlesForDate(ctx, indexID, d, iv)
			if err != nil {
				return total, err
			}
			if count > 0 {
				continue
			}
		}

		rows, err := fetchCandles(ctx, client, indexID, d, iv)
		if err != nil {
			return total, fmt.Errorf("fetch %s candles interval=%d for %s: %w",
				indexID, iv, d.Format(time.DateOnly), err)
		}
		if len(rows) == 0 {
			continue
		}
		if err := storage.DeleteIndexCandlesFor(ctx, indexID, d, iv); err != nil {
			return total, fmt.Errorf("delete %s candles interval=%d for %s: %w",
				indexID, iv, d.Format(time.DateOnly), err)
		}
		if err := storage.StoreIndexCandles(ctx, rows); err != nil {
			return total, fmt.Errorf("store %s candles interval=%d for %s: %w",
				indexID, iv, d.Format(time.DateOnly), err)
		}
		total += len(rows)
	}
	return total, nil
}

// loadDayWeights догружает веса индексов за дату (точечно по индексу).
func loadDayWeights(
	ctx context.Context,
	storage *store.Store,
	client *http.Client,
	indices []string,
	d time.Time,
	force bool,
	lastByIndex map[string]time.Time,
) (int, error) {
	var (
		mu    sync.Mutex
		total int
	)
	gr, ctx := errgroup.WithContext(ctx)
	gr.SetLimit(indexWorkers)
	for _, indexID := range indices {
		indexID := indexID
		gr.Go(func() error {
			reload := force || sameDay(lastByIndex[indexID], d)
			if !reload {
				count, err := storage.CountIndexWeightsForDate(ctx, indexID, d)
				if err != nil {
					return err
				}
				if count > 0 {
					return nil
				}
			}

			rows, err := fetchWeights(ctx, client, indexID, d)
			if err != nil {
				return fmt.Errorf("fetch %s weights for %s: %w", indexID, d.Format(time.DateOnly), err)
			}
			if len(rows) == 0 {
				return nil
			}
			if err := storage.DeleteIndexWeightsFor(ctx, indexID, d); err != nil {
				return fmt.Errorf("delete %s weights for %s: %w", indexID, d.Format(time.DateOnly), err)
			}
			if err := storage.StoreIndexWeights(ctx, rows); err != nil {
				return fmt.Errorf("store %s weights for %s: %w", indexID, d.Format(time.DateOnly), err)
			}
			mu.Lock()
			total += len(rows)
			mu.Unlock()
			return nil
		})
	}
	if err := gr.Wait(); err != nil {
		return total, err
	}
	return total, nil
}

// watchIndexData периодически (раз в час) перезаливает текущий день.
func watchIndexData(ctx context.Context, storage *store.Store, client *http.Client, indices []string) error {
	var tracker watch.Tracker
	return watch.RunHourly(ctx, func(ctx context.Context) error {
		now := time.Now()
		for _, d := range tracker.Days(now) {
			if _, err := loadDayCandles(ctx, storage, client, indices, d, true, true, nil); err != nil {
				return err
			}
			if _, err := loadDayWeights(ctx, storage, client, indices, d, true, nil); err != nil {
				return err
			}
			fmt.Printf("watch index data: %s reloaded\n", d.Format(time.DateOnly))
		}
		tracker.Commit(now)
		return nil
	})
}

// fetchCandles выкачивает все страницы свечей одного индекса за дату.
func fetchCandles(ctx context.Context, client *http.Client, indexID string, date time.Time, interval uint16) ([]store.IndexCandle, error) {
	dateStr := date.Format(time.DateOnly)
	var rows []store.IndexCandle

	for start := 0; ; start += candlesPageSize {
		u := fmt.Sprintf(candlesURLFmt, url.PathEscape(indexID)) + "?" + url.Values{
			"from":     {dateStr},
			"till":     {dateStr},
			"interval": {strconv.FormatUint(uint64(interval), 10)},
			"iss.json": {"extended"},
			"iss.meta": {"off"},
			"iss.only": {"candles"},
			"start":    {strconv.Itoa(start)},
		}.Encode()

		body, err := httpjson.GetJSON(ctx, client, u)
		if err != nil {
			return nil, err
		}

		page := gjson.GetBytes(body, "1.candles")
		if !page.IsArray() {
			return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
		}

		items := page.Array()
		for _, r := range items {
			begin, err := parseMSK(r.Get("begin").String())
			if err != nil {
				return nil, fmt.Errorf("parse begin %q: %w", r.Get("begin").String(), err)
			}
			end, err := parseMSK(r.Get("end").String())
			if err != nil {
				return nil, fmt.Errorf("parse end %q: %w", r.Get("end").String(), err)
			}
			rows = append(rows, store.IndexCandle{
				IndexID:  indexID,
				Interval: interval,
				Time:     begin,
				End:      end,
				Open:     r.Get("open").Float(),
				Close:    r.Get("close").Float(),
				High:     r.Get("high").Float(),
				Low:      r.Get("low").Float(),
				Value:    r.Get("value").Float(),
				Volume:   r.Get("volume").Float(),
			})
		}

		if len(items) < candlesPageSize {
			break
		}
	}
	return rows, nil
}

// fetchWeights выкачивает все страницы весов одного индекса за дату.
func fetchWeights(ctx context.Context, client *http.Client, indexID string, date time.Time) ([]store.IndexWeight, error) {
	dateStr := date.Format(time.DateOnly)
	var rows []store.IndexWeight

	for start := 0; ; start += weightsPageSize {
		u := fmt.Sprintf(analyticsURLFmt, url.PathEscape(indexID)) + "?" + url.Values{
			"date":     {dateStr},
			"iss.json": {"extended"},
			"iss.meta": {"off"},
			"iss.only": {"analytics"},
			"start":    {strconv.Itoa(start)},
			"limit":    {strconv.Itoa(weightsPageSize)},
		}.Encode()

		body, err := httpjson.GetJSON(ctx, client, u)
		if err != nil {
			return nil, err
		}

		page := gjson.GetBytes(body, "1.analytics")
		if !page.IsArray() {
			return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
		}

		items := page.Array()
		for _, r := range items {
			tradedate, err := time.ParseInLocation(time.DateOnly, r.Get("tradedate").String(), tz.MSK)
			if err != nil {
				return nil, fmt.Errorf("parse tradedate %q: %w", r.Get("tradedate").String(), err)
			}
			sessionDate, err := time.ParseInLocation(time.DateOnly, r.Get("trade_session_date").String(), tz.MSK)
			if err != nil {
				sessionDate = tradedate
			}
			rows = append(rows, store.IndexWeight{
				IndexID:          indexID,
				TradeDate:        tradedate,
				Ticker:           r.Get("ticker").String(),
				ShortName:        r.Get("shortnames").String(),
				SecID:            r.Get("secids").String(),
				Weight:           r.Get("weight").Float(),
				TradingSession:   int32(r.Get("tradingsession").Int()),
				TradeSessionDate: sessionDate,
			})
		}

		if len(items) < weightsPageSize {
			break
		}
	}
	return rows, nil
}

func parseMSK(s string) (time.Time, error) {
	return time.ParseInLocation(time.DateTime, s, tz.MSK)
}

// minLastDate возвращает самую раннюю последнюю дату среди индексов, у которых
// есть свежие данные (не старше cutoff). ok=false, если таких нет.
func minLastDate(last map[string]time.Time, indices []string, cutoff time.Time) (time.Time, bool) {
	var minT time.Time
	for _, idx := range indices {
		t := last[idx]
		if t.IsZero() || t.Before(cutoff) {
			continue
		}
		if minT.IsZero() || t.Before(minT) {
			minT = t
		}
	}
	if minT.IsZero() {
		return time.Time{}, false
	}
	return minT, true
}

func dayMSK(t time.Time) time.Time {
	n := t.In(tz.MSK)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, tz.MSK)
}

// sameDay сравнивает две даты по календарному дню (MSK).
func sameDay(a, b time.Time) bool {
	return !a.IsZero() && !b.IsZero() && a.In(tz.MSK).Format(time.DateOnly) == b.In(tz.MSK).Format(time.DateOnly)
}
