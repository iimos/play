// Package indexdata загружает данные по индексам MOEX (сам индекс, не фьючерс):
// свечи индекса (OHLC + оборот) и веса бумаг в индексе.
package indexdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	// ISS отдаёт 10-минутные свечи только за последние ~60 дней, глубже
	// интрадей недоступен и при релоаде удаляется безвозвратно.
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
}

func Load(ctx context.Context, opts LoadOptions) error {
	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	infos, err := moexindex.Discover(ctx, client)
	if err != nil {
		return fmt.Errorf("discover indices: %w", err)
	}
	indices := indexIDs(infos)

	// Последняя загруженная дата по каждому индексу: её всегда перезаливаем.
	lastCandle, err := storage.LastIndexCandleDates(ctx)
	if err != nil {
		return err
	}
	lastWeight, err := storage.LastIndexWeightsDates(ctx)
	if err != nil {
		return err
	}
	lastDates := map[string]struct{}{}
	for _, t := range lastCandle {
		lastDates[t.In(tz.MSK).Format(time.DateOnly)] = struct{}{}
	}
	for _, t := range lastWeight {
		lastDates[t.In(tz.MSK).Format(time.DateOnly)] = struct{}{}
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

	fmt.Printf("Loading index data (%d indices) from %s to %s\n",
		len(indices), start.Format(time.DateOnly), end.Format(time.DateOnly))

	intradayFrom := end.AddDate(0, 0, -intradayDays)

	for d := end; !d.Before(start); d = d.AddDate(0, 0, -1) {
		reload := opts.ForceReload
		if !reload {
			candleCount, err := storage.CountIndexCandlesPartition(ctx, d)
			if err != nil {
				return err
			}
			weightCount, err := storage.CountIndexWeightsPartition(ctx, d)
			if err != nil {
				return err
			}
			_, isLast := lastDates[d.In(tz.MSK).Format(time.DateOnly)]
			reload = candleCount == 0 || weightCount == 0 || isLast
		}
		if !reload {
			continue
		}

		withIntraday := !d.Before(intradayFrom)

		candles, weights, err := loadDay(ctx, storage, client, infos, d, withIntraday)
		if err != nil {
			return err
		}
		fmt.Printf("> %s: candles %d, weights %d\n", d.Format(time.DateOnly), candles, weights)
	}

	if opts.Watch {
		return watchIndexData(ctx, storage, client, infos)
	}
	return nil
}

// loadDay перезагружает день целиком: параллельно качает активные индексы в
// память, дропает партицию дня и вставляет одним батчем.
func loadDay(
	ctx context.Context,
	storage *store.Store,
	client *http.Client,
	infos []moexindex.Info,
	d time.Time,
	withIntraday bool,
) (int, int, error) {
	// interval=24 грузим всегда, interval=10 — только в пределах intradayDays.
	// Дневная свеча MOEX идёт от 00:00 до 23:59 и не содержит границ сессии,
	// поэтому окно торгов индекса (IndexIntradaySession) строится по 10-минутным,
	// а глубже intradayDays используется фолбэк основной сессии.
	intervals := []uint16{24}
	if withIntraday {
		intervals = append(intervals, intradayInterval)
	}

	active := activeInfos(infos, d)
	if len(active) == 0 {
		return 0, 0, nil
	}

	var (
		mu      sync.Mutex
		candles []store.IndexCandle
		weights []store.IndexWeight
	)
	gr, gctx := errgroup.WithContext(ctx)
	gr.SetLimit(indexWorkers)
	for _, info := range active {
		info := info
		gr.Go(func() error {
			var localCandles []store.IndexCandle
			for _, iv := range intervals {
				rows, err := fetchCandles(gctx, client, info.ID, d, iv)
				if err != nil {
					return fmt.Errorf("fetch %s candles interval=%d for %s: %w",
						info.ID, iv, d.Format(time.DateOnly), err)
				}
				localCandles = append(localCandles, rows...)
			}
			rows, err := fetchWeights(gctx, client, info.ID, d)
			if err != nil {
				return fmt.Errorf("fetch %s weights for %s: %w", info.ID, d.Format(time.DateOnly), err)
			}
			mu.Lock()
			candles = append(candles, localCandles...)
			weights = append(weights, rows...)
			mu.Unlock()
			return nil
		})
	}
	if err := gr.Wait(); err != nil {
		return 0, 0, err
	}

	downloaded := make(map[string]struct{}, len(active))
	for _, info := range active {
		downloaded[info.ID] = struct{}{}
	}
	if err := warnMissingIndices(ctx, storage, d, downloaded); err != nil {
		return 0, 0, err
	}

	// DROP PARTITION сносит весь день, включая interval=10. Для дней старше
	// intradayDays он уже не перекачивается (ISS его не отдаёт), так что лежавший
	// там интрадей теряется безвозвратно. Это сознательно: interval=10 нужен лишь
	// для окна сессии при build-index-super, а старые дни либо не пересобираются,
	// либо обходятся фолбэком 09:00–19:00.
	if len(candles) > 0 {
		if err := storage.DropIndexCandlesPartition(ctx, d); err != nil {
			return 0, 0, fmt.Errorf("drop candles partition %s: %w", d.Format(time.DateOnly), err)
		}
		if err := storage.StoreIndexCandles(ctx, candles); err != nil {
			return 0, 0, fmt.Errorf("store candles for %s: %w", d.Format(time.DateOnly), err)
		}
	}
	if len(weights) > 0 {
		if err := storage.DropIndexWeightsPartition(ctx, d); err != nil {
			return 0, 0, fmt.Errorf("drop weights partition %s: %w", d.Format(time.DateOnly), err)
		}
		if err := storage.StoreIndexWeights(ctx, weights); err != nil {
			return 0, 0, fmt.Errorf("store weights for %s: %w", d.Format(time.DateOnly), err)
		}
	}
	return len(candles), len(weights), nil
}

// warnMissingIndices предупреждает об индексах, у которых есть строки за день,
// но которых нет в скачанном наборе: DROP PARTITION удалит и их строки.
func warnMissingIndices(ctx context.Context, storage *store.Store, d time.Time, downloaded map[string]struct{}) error {
	candleIDs, err := storage.PartitionIndexCandleIndexIDs(ctx, d)
	if err != nil {
		return err
	}
	weightIDs, err := storage.PartitionIndexWeightIndexIDs(ctx, d)
	if err != nil {
		return err
	}
	var missing []string
	for _, id := range append(candleIDs, weightIDs...) {
		if _, ok := downloaded[id]; ok {
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) > 0 {
		fmt.Printf("> %s: warning: индексы вне загружаемого набора, их строки дня будут удалены: %v\n",
			d.Format(time.DateOnly), missing)
	}
	return nil
}

// watchIndexData периодически (раз в час) перезаливает текущий день.
func watchIndexData(ctx context.Context, storage *store.Store, client *http.Client, infos []moexindex.Info) error {
	var tracker watch.Tracker
	return watch.RunHourly(ctx, func(ctx context.Context) error {
		now := time.Now()
		for _, d := range tracker.Days(now) {
			if _, _, err := loadDay(ctx, storage, client, infos, d, true); err != nil {
				return err
			}
			fmt.Printf("watch index data: %s reloaded\n", d.Format(time.DateOnly))
		}
		tracker.Commit(now)
		return nil
	})
}

// indexIDs возвращает идентификаторы индексов, пропуская пустые.
func indexIDs(infos []moexindex.Info) []string {
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.ID != "" {
			ids = append(ids, info.ID)
		}
	}
	return ids
}

// activeInfos возвращает индексы, действовавшие на дату d.
func activeInfos(infos []moexindex.Info, d time.Time) []moexindex.Info {
	res := make([]moexindex.Info, 0, len(infos))
	for _, info := range infos {
		if info.ID != "" && moexindex.ActiveOn(info, d) {
			res = append(res, info)
		}
	}
	return res
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
