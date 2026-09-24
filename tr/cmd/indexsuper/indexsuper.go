// Package indexsuper синтезирует 5-минутные суперсвечи индексов из суперсвечей
// акций (super_eq) и весов бумаг в индексе (index_weights).
package indexsuper

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/moexindex"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
	"golang.org/x/sync/errgroup"
)

const (
	// usdRubSecID — код USD/RUB в super_fx для пересчета USD-индексов (RTS*).
	usdRubSecID = "USD000UTSTOM"
	// cnyRubSecID — код CNY/RUB в super_fx для индексов в юанях.
	cnyRubSecID = "CNYRUB_TOM"

	// fallbackStartHour/fallbackEndHour — границы основной сессии, если
	// интрадей-свечи индекса недоступны (историю грузим только за 60 дней).
	fallbackStartHour = 9
	fallbackEndHour   = 19

	// staleDays — насколько назад искать последнюю цену бумаги, если в
	// референсный день сделок не было (индекс использует последнюю цену).
	staleDays = 30

	// validationThresholdPct — порог расхождения синтеза и фактического закрытия.
	validationThresholdPct = 0.5

	// indexWorkers — параллелизм сборки индексов.
	indexWorkers = 8
)

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	Watch       bool
	Indices     []string
}

func Build(ctx context.Context, opts LoadOptions) error {
	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	indices, err := moexindex.Resolve(ctx, client, opts.Indices)
	if err != nil {
		return fmt.Errorf("resolve indices: %w", err)
	}

	// Валюта номинала нужна для корректного синтеза валютных индексов (RUBMI, RTS*, IMOEXCNY, ...).
	currencies, err := moexindex.Currencies(ctx, client)
	if err != nil {
		fmt.Printf("warning: не удалось получить валюты индексов, использую эвристику: %v\n", err)
		currencies = nil
	}

	lastByIndex := map[string]time.Time{}
	for _, idx := range indices {
		if t, err := storage.GetLastSuperIndexDateFor(ctx, idx); err != nil {
			return err
		} else {
			lastByIndex[idx] = t
		}
	}

	start := opts.StartDate
	end := opts.EndDate
	if end.IsZero() {
		end = time.Now().In(tz.MSK)
	}
	if start.IsZero() {
		start = end.AddDate(0, 0, -10)
		if minT, ok := minLastDate(lastByIndex, indices, end.AddDate(0, 0, -staleDays)); ok {
			start = minT
		}
	}
	start = dayMSK(start)
	end = dayMSK(end)

	fmt.Printf("Building super_index (%v) from %s to %s\n",
		indices, start.Format(time.DateOnly), end.Format(time.DateOnly))

	for d := end; !d.Before(start); d = d.AddDate(0, 0, -1) {
		rows, built, err := buildDay(ctx, storage, indices, d, opts.ForceReload, lastByIndex, currencies)
		if err != nil {
			return fmt.Errorf("build super_index for %s: %w", d.Format(time.DateOnly), err)
		}

		if len(rows) == 0 {
			fmt.Printf("> %s super_index: NO DATA\n", d.Format(time.DateOnly))
			continue
		}

		// Точечно заменяем данные только тех индексов, что пересобраны.
		for _, indexID := range built {
			if err := storage.DeleteSuperIndexFor(ctx, indexID, d); err != nil {
				return fmt.Errorf("delete super_index %s %s: %w", indexID, d.Format(time.DateOnly), err)
			}
		}
		if err := storage.StoreSuperIndex(ctx, rows); err != nil {
			return fmt.Errorf("store super_index for %s: %w", d.Format(time.DateOnly), err)
		}
		fmt.Printf("> %s super_index: BUILT %d rows (%v)\n", d.Format(time.DateOnly), len(rows), built)

		runtime.GC()
	}

	if opts.Watch {
		var tracker watch.Tracker
		return watch.RunHourly(ctx, func(ctx context.Context) error {
			now := time.Now()
			for _, d := range tracker.Days(now) {
				rows, built, err := buildDay(ctx, storage, indices, d, true, nil, currencies)
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					continue
				}
				for _, indexID := range built {
					if err := storage.DeleteSuperIndexFor(ctx, indexID, d); err != nil {
						return err
					}
				}
				if err := storage.StoreSuperIndex(ctx, rows); err != nil {
					return err
				}
				fmt.Printf("watch super_index: %s rebuilt\n", d.Format(time.DateOnly))
			}
			tracker.Commit(now)
			return nil
		})
	}
	return nil
}

// buildDay возвращает строки super_index за дату и список индексов, для которых
// они построены. lastByIndex используется для пропуска уже загруженных дат.
func buildDay(
	ctx context.Context,
	storage *store.Store,
	indices []string,
	d time.Time,
	force bool,
	lastByIndex map[string]time.Time,
	currencies map[string]string,
) ([]store.SuperIndexRow, []string, error) {
	// Определяем, какие индексы нужно строить.
	var todo []string
	for _, indexID := range indices {
		reload := force || sameDay(lastByIndex[indexID], d)
		if !reload {
			count, err := storage.CountSuperIndexForDate(ctx, indexID, d)
			if err != nil {
				return nil, nil, err
			}
			if count > 0 {
				continue
			}
		}
		todo = append(todo, indexID)
	}
	if len(todo) == 0 {
		return nil, nil, nil
	}

	// Бары бумаг нужны всем индексам — грузим один раз на дату (весь день,
	// включая premarket/вечернюю сессию); каждый индекс затем фильтрует своё окно.
	dayStart := dayMSK(d)
	dayEnd := dayStart.AddDate(0, 0, 1)

	// Бары бумаг нужны всем индексам — грузим один раз на дату.
	bars, err := storage.StockBars(ctx, dayStart, dayEnd)
	if err != nil {
		return nil, nil, err
	}
	barsBySec := map[string][]store.StockBar{}
	for _, b := range bars {
		barsBySec[b.SecID] = append(barsBySec[b.SecID], b)
	}

	var (
		mu    sync.Mutex
		out   []store.SuperIndexRow
		built []string
	)
	closes := newClosesCache()
	gr, ctx := errgroup.WithContext(ctx)
	gr.SetLimit(indexWorkers)
	for _, indexID := range todo {
		indexID := indexID
		gr.Go(func() error {
			rows, err := buildIndexDay(ctx, storage, indexID, d, barsBySec, closes, currencies)
			if err != nil {
				return fmt.Errorf("%s: %w", indexID, err)
			}
			if len(rows) == 0 {
				return nil
			}
			mu.Lock()
			out = append(out, rows...)
			built = append(built, indexID)
			mu.Unlock()
			return nil
		})
	}
	if err := gr.Wait(); err != nil {
		return nil, nil, err
	}
	return out, built, nil
}

// closesCache кэширует последние цены бумаг по референсной дате, чтобы не
// выполнять один и тот же запрос к super_eq для каждого индекса.
type closesCache struct {
	mu sync.Mutex
	m  map[string]map[string]float64
}

func newClosesCache() *closesCache {
	return &closesCache{m: map[string]map[string]float64{}}
}

func (c *closesCache) get(ctx context.Context, s *store.Store, date, cutoff time.Time) (map[string]float64, error) {
	key := date.Format(time.DateOnly)
	c.mu.Lock()
	if v, ok := c.m[key]; ok {
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	v, err := s.StockMainCloses(ctx, cutoff.AddDate(0, 0, -staleDays), cutoff)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.m[key] = v
	c.mu.Unlock()
	return v, nil
}

type constituent struct {
	secID  string
	weight float64
	ref    float64
}

// buildIndexDay синтезирует суперсвечи одного индекса за дату.
func buildIndexDay(
	ctx context.Context,
	storage *store.Store,
	indexID string,
	d time.Time,
	barsBySec map[string][]store.StockBar,
	closes *closesCache,
	currencies map[string]string,
) ([]store.SuperIndexRow, error) {
	// Индекс не торговался в этот день — фактической дневной свечи нет.
	dayClose, ok, err := storage.IndexDailyClose(ctx, indexID, d)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	prevDate, err := storage.PrevTradingDate(ctx, indexID, d)
	if err != nil {
		return nil, err
	}
	if prevDate.IsZero() {
		return nil, nil
	}

	anchor, ok, err := storage.IndexDailyClose(ctx, indexID, prevDate)
	if err != nil {
		return nil, err
	}
	if !ok || anchor <= 0 {
		fmt.Printf(" [%s: нет якоря за %s, пропуск]", indexID, prevDate.Format(time.DateOnly))
		return nil, nil
	}

	weights, err := storage.IndexWeightsForDate(ctx, indexID, prevDate)
	if err != nil {
		return nil, err
	}
	if len(weights) == 0 {
		fmt.Printf(" [%s: нет весов за %s, пропуск]", indexID, prevDate.Format(time.DateOnly))
		return nil, nil
	}

	// Референсные цены — последняя ненулевая цена до закрытия прошлого дня.
	cutoff := atHour(prevDate, fallbackEndHour)
	prevCloses, err := closes.get(ctx, storage, prevDate, cutoff)
	if err != nil {
		return nil, err
	}

	// Окно торгов индекса: из интрадей-свечей, иначе — основная сессия.
	sessStart, sessEnd, ok, err := storage.IndexIntradaySession(ctx, indexID, d)
	if err != nil {
		return nil, err
	}
	if !ok {
		sessStart = atHour(d, fallbackStartHour)
		sessEnd = atHour(d, fallbackEndHour)
	}

	// Пересчет в валюту: USD-индексы (RTS*) и индекс в юанях (IMOEXCNY).
	fxSec := fxSecIDFor(indexID, currencies)
	useFX := fxSec != ""
	var fxPrev float64
	fxLast := 0.0
	fxByTime := map[int64]float64{}
	if useFX {
		fxPrev, ok, err = storage.FXMainClose(ctx, fxSec, prevDate, cutoff)
		if err != nil {
			return nil, err
		}
		if !ok || fxPrev <= 0 {
			fmt.Printf(" [%s: нет курса %s за %s, пропуск]", indexID, fxSec, prevDate.Format(time.DateOnly))
			return nil, nil
		}
		fxLast = fxPrev
		series, err := storage.FXCloseSeries(ctx, fxSec, sessStart, sessEnd)
		if err != nil {
			return nil, err
		}
		for _, p := range series {
			fxByTime[p.Time.Unix()] = p.Close
		}
	}

	seen := make(map[string]struct{}, len(weights))
	constituents := make([]constituent, 0, len(weights))
	for _, w := range weights {
		secID := w.SecID
		if secID == "" {
			secID = w.Ticker
		}
		if _, dup := seen[secID]; dup {
			continue
		}
		ref := prevCloses[secID]
		if ref <= 0 {
			continue
		}
		seen[secID] = struct{}{}
		constituents = append(constituents, constituent{secID: secID, weight: w.Weight / 100, ref: ref})
	}
	if len(constituents) == 0 {
		fmt.Printf(" [%s: нет цен бумаг за %s, пропуск]", indexID, prevDate.Format(time.DateOnly))
		return nil, nil
	}

	// Индексируем бары бумаг по 5-минутным слотам внутри окна сессии.
	// Ключ — Unix-время: сравнение time.Time как ключа учитывает таймзону.
	barsAt := make(map[string]map[int64]store.StockBar, len(constituents))
	for _, c := range constituents {
		m := map[int64]store.StockBar{}
		for _, b := range barsBySec[c.secID] {
			if b.Time.Before(sessStart) || !b.Time.Before(sessEnd) {
				continue
			}
			m[b.Time.Unix()] = b
		}
		barsAt[c.secID] = m
	}

	// Последние известные цены (forward fill); инициализируем референсом,
	// чтобы бумага без сделок давала нулевой вклад.
	type price struct{ open, high, low, close, vwap, vwapB, vwapS float64 }
	last := make(map[string]price, len(constituents))
	for _, c := range constituents {
		last[c.secID] = price{c.ref, c.ref, c.ref, c.ref, c.ref, c.ref, c.ref}
	}

	sessStart = sessStart.Truncate(5 * time.Minute)

	var rows []store.SuperIndexRow
	for t := sessStart; t.Before(sessEnd); t = t.Add(5 * time.Minute) {
		if useFX {
			if v, ok := fxByTime[t.Unix()]; ok && v > 0 {
				fxLast = v
			}
		}

		type presentBar struct {
			bar store.StockBar
			c   constituent
		}
		var present []presentBar

		for _, c := range constituents {
			p := last[c.secID]
			if b, ok := barsAt[c.secID][t.Unix()]; ok {
				if b.PrOpen > 0 {
					p.open = float64(b.PrOpen)
				}
				if b.PrHigh > 0 {
					p.high = float64(b.PrHigh)
				}
				if b.PrLow > 0 {
					p.low = float64(b.PrLow)
				}
				if b.PrClose > 0 {
					p.close = float64(b.PrClose)
				}
				if b.PrVWAP > 0 {
					p.vwap = float64(b.PrVWAP)
				}
				if b.PrVWAPB > 0 {
					p.vwapB = float64(b.PrVWAPB)
				}
				if b.PrVWAPS > 0 {
					p.vwapS = float64(b.PrVWAPS)
				}
				present = append(present, presentBar{bar: b, c: c})
			} else {
				// Нет сделок в интервале — цена не меняется: open=high=low=close.
				p.open = p.close
				p.high = p.close
				p.low = p.close
				p.vwap = p.close
				p.vwapB = p.close
				p.vwapS = p.close
			}
			last[c.secID] = p
		}

		fxRatio := 1.0
		if useFX && fxLast > 0 {
			fxRatio = fxLast / fxPrev
		}
		level := func(pick func(price) float64) float64 {
			sum := 0.0
			for _, c := range constituents {
				r := pick(last[c.secID]) / c.ref
				if useFX {
					r /= fxRatio
				}
				sum += c.weight * (r - 1)
			}
			return anchor * (1 + sum)
		}

		row := store.SuperIndexRow{
			IndexID:      indexID,
			Time:         t,
			RefClose:     anchor,
			PrOpen:       float32(level(func(p price) float64 { return p.open })),
			PrHigh:       float32(level(func(p price) float64 { return p.high })),
			PrLow:        float32(level(func(p price) float64 { return p.low })),
			PrClose:      float32(level(func(p price) float64 { return p.close })),
			PrVWAP:       float32(level(func(p price) float64 { return p.vwap })),
			PrVWAPB:      float32(level(func(p price) float64 { return p.vwapB })),
			PrVWAPS:      float32(level(func(p price) float64 { return p.vwapS })),
			Constituents: uint16(len(constituents)),
		}
		if row.PrOpen != 0 {
			row.PrChange = 100 * (row.PrClose - row.PrOpen) / row.PrOpen
		}

		// Агрегаты по бумагам, торговавшимся в этот интервал.
		var (
			val, valB, valS, putValB, putValS, cancelValB, cancelValS, putVal, cancelVal float64
			vol, volB, volS, putVolB, putVolS, cancelVolB, cancelVolS, putVol, cancelVol uint64
			trades, tradesB, tradesS                                                     uint64
			putOrdersB, putOrdersS, putOrders                                            uint64
			cancelOrdersB, cancelOrdersS, cancelOrders                                   uint64
			levelsB, levelsS                                                             uint64
			prStd                                                                        float64
		)
		totalVal := 0.0
		for _, p := range present {
			totalVal += float64(p.bar.Val)
		}
		weightOf := func(p presentBar) float64 {
			if totalVal > 0 {
				return float64(p.bar.Val)
			}
			return p.c.weight
		}

		var (
			wSum                                         float64
			spreadBBO, spreadLV10, spread1Mio            float64
			imbVolBBO, imbValBBO, imbVol, imbVal         float64
			vwapB, vwapS, vwapB1Mio, vwapS1Mio           float64
			putVWAPB, putVWAPS, cancelVWAPB, cancelVWAPS float64
		)

		for _, p := range present {
			b := p.bar
			val += float64(b.Val)
			valB += float64(b.ValB)
			valS += float64(b.ValS)
			vol += uint64(b.Vol)
			volB += b.VolB
			volS += b.VolS
			trades += uint64(b.Trades)
			tradesB += uint64(b.TradesB)
			tradesS += uint64(b.TradesS)
			putValB += float64(b.PutValB)
			putValS += float64(b.PutValS)
			putVolB += uint64(b.PutVolB)
			putVolS += uint64(b.PutVolS)
			putVal += float64(b.PutVal)
			putVol += uint64(b.PutVol)
			putOrdersB += uint64(b.PutOrdersB)
			putOrdersS += uint64(b.PutOrdersS)
			putOrders += uint64(b.PutOrders)
			cancelValB += float64(b.CancelValB)
			cancelValS += float64(b.CancelValS)
			cancelVolB += uint64(b.CancelVolB)
			cancelVolS += b.CancelVolS
			cancelVol += b.CancelVol
			cancelVal += float64(b.CancelVal)
			cancelOrdersB += uint64(b.CancelOrdersB)
			cancelOrdersS += uint64(b.CancelOrdersS)
			cancelOrders += b.CancelOrders
			levelsB += uint64(b.LevelsB)
			levelsS += uint64(b.LevelsS)
			prStd += p.c.weight * float64(b.PrStd)

			w := weightOf(p)
			wSum += w
			spreadBBO += w * float64(b.SpreadBBO)
			spreadLV10 += w * float64(b.SpreadLV10)
			spread1Mio += w * float64(b.Spread1Mio)
			imbVolBBO += w * float64(b.ImbalanceVolBBO)
			imbValBBO += w * float64(b.ImbalanceValBBO)
			imbVol += w * float64(b.ImbalanceVol)
			imbVal += w * float64(b.ImbalanceVal)
			vwapB += w * float64(b.VWAPB)
			vwapS += w * float64(b.VWAPS)
			vwapB1Mio += w * float64(b.VWAPB1Mio)
			vwapS1Mio += w * float64(b.VWAPS1Mio)
			putVWAPB += w * float64(b.PutVWAPB)
			putVWAPS += w * float64(b.PutVWAPS)
			cancelVWAPB += w * float64(b.CancelVWAPB)
			cancelVWAPS += w * float64(b.CancelVWAPS)
		}

		row.Val = val
		row.ValB = valB
		row.ValS = valS
		row.Vol = vol
		row.VolB = volB
		row.VolS = volS
		row.Trades = trades
		row.TradesB = tradesB
		row.TradesS = tradesS
		row.PutValB = putValB
		row.PutValS = putValS
		row.PutVolB = putVolB
		row.PutVolS = putVolS
		row.PutVal = putVal
		row.PutVol = putVol
		row.PutOrdersB = putOrdersB
		row.PutOrdersS = putOrdersS
		row.PutOrders = putOrders
		row.CancelValB = cancelValB
		row.CancelValS = cancelValS
		row.CancelVolB = cancelVolB
		row.CancelVolS = cancelVolS
		row.CancelVol = cancelVol
		row.CancelVal = cancelVal
		row.CancelOrdersB = cancelOrdersB
		row.CancelOrdersS = cancelOrdersS
		row.CancelOrders = cancelOrders
		row.LevelsB = levelsB
		row.LevelsS = levelsS
		row.PrStd = float32(prStd)

		if valB+valS > 0 {
			row.Disb = float32((valB - valS) / (valB + valS))
		}

		if wSum > 0 {
			row.SpreadBBO = float32(spreadBBO / wSum)
			row.SpreadLV10 = float32(spreadLV10 / wSum)
			row.Spread1Mio = float32(spread1Mio / wSum)
			row.ImbalanceVolBBO = float32(imbVolBBO / wSum)
			row.ImbalanceValBBO = float32(imbValBBO / wSum)
			row.ImbalanceVol = float32(imbVol / wSum)
			row.ImbalanceVal = float32(imbVal / wSum)
			row.VWAPB = float32(vwapB / wSum)
			row.VWAPS = float32(vwapS / wSum)
			row.VWAPB1Mio = float32(vwapB1Mio / wSum)
			row.VWAPS1Mio = float32(vwapS1Mio / wSum)
			row.PutVWAPB = float32(putVWAPB / wSum)
			row.PutVWAPS = float32(putVWAPS / wSum)
			row.CancelVWAPB = float32(cancelVWAPB / wSum)
			row.CancelVWAPS = float32(cancelVWAPS / wSum)
		}

		rows = append(rows, row)
	}

	validate(rows, indexID, d, dayClose)
	return rows, nil
}

// validate сверяет синтезированное закрытие дня с фактической свечой индекса.
func validate(rows []store.SuperIndexRow, indexID string, d time.Time, actual float64) {
	if len(rows) == 0 || actual <= 0 {
		return
	}
	got := float64(rows[len(rows)-1].PrClose)
	diffPct := 100 * math.Abs(got-actual) / actual
	if diffPct > validationThresholdPct {
		fmt.Printf(" [%s %s: расхождение синтеза %.2f%% (синтез %.2f, факт %.2f)]",
			indexID, d.Format(time.DateOnly), diffPct, got, actual)
	}
}

func atHour(d time.Time, hour int) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, tz.MSK)
}

// fxSecIDFor возвращает код валюты в super_fx, в которой номинирован индекс,
// или пустую строку для рублёвых индексов. Валюта берётся из ISS (CURRENCYID);
// если она неизвестна, используется эвристика по коду (RTS* и RUBMI — USD,
// IMOEXCNY — CNY).
func fxSecIDFor(indexID string, currencies map[string]string) string {
	if currencies != nil {
		switch strings.ToUpper(currencies[indexID]) {
		case "USD":
			return usdRubSecID
		case "CNY":
			return cnyRubSecID
		case "RUB":
			return ""
		}
	}
	switch {
	case strings.HasPrefix(indexID, "RTS"), indexID == "RUBMI":
		return usdRubSecID
	case indexID == "IMOEXCNY":
		return cnyRubSecID
	}
	return ""
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

func sameDay(a, b time.Time) bool {
	return !a.IsZero() && !b.IsZero() && a.In(tz.MSK).Format(time.DateOnly) == b.In(tz.MSK).Format(time.DateOnly)
}
