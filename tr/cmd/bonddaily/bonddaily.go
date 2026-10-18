package bonddaily

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/httpjson"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
	"github.com/tidwall/gjson"
)

const historyURL = "https://iss.moex.com/iss/history/engines/stock/markets/bonds/boards/%s/securities.json"

var boards = [...]string{"TQCB", "TQOB", "TQDB"}

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

	lastTableDate, err := storage.GetLastBondDailyDate(ctx)
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

	fmt.Printf("Loading bond daily data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	client := &http.Client{Timeout: 30 * time.Second}

	// Автостарт взят из таблицы, а не из --start: идём от старых дней к новым.
	// Тогда обрыв загрузки оставляет последнюю дату таблицы перед дырой, и
	// повторный запуск без флагов продолжает с неё (см. watch.DateRange).
	asc := opts.StartDate.IsZero()

	// Отмену контекста (Ctrl-C) уважаем при скачивании, но не между DROP
	// PARTITION и INSERT: иначе прерывание оставит день пустым.
	wctx := context.WithoutCancel(ctx)

	var emptyDays watch.EmptyDays
	emptyDays.SkipWeekends() // по сб/вс источник обычно пуст; торговые субботы печатаются отдельно
	defer emptyDays.Report("bond_daily")

	for _, d := range watch.DayRange(start, end, asc, opts.StartDate.IsZero()) {
		dateStr := d.Format(time.DateOnly)
		msg, err := func() (string, error) {
			dateStr := d.Format(time.DateOnly)
			var parts []string

			isLastTableDate := !lastTableDate.IsZero() && dateStr == lastTableDate.Format(time.DateOnly)
			shouldReload := opts.ForceReload || isLastTableDate

			if !shouldReload {
				count, err := storage.CountBondDailyForDate(ctx, d)
				if err != nil {
					return "", fmt.Errorf("count bond daily for date %s: %w", dateStr, err)
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

			// Сначала качаем данные и только потом удаляем партицию: иначе сбой сети
			// или пустой ответ стирает уже загруженный день.
			rows, err := fetchBondDaily(ctx, client, d)
			if err != nil {
				return "", fmt.Errorf("fetch bond daily for date %s: %w", dateStr, err)
			}
			if len(rows) == 0 {
				if asc {
					emptyDays.Add(d)
				}
				parts = append(parts, "NO DATA; partition untouched")
				return strings.Join(parts, "; "), nil
			}
			if shouldReload {
				if err := storage.DeleteBondDailyPartition(wctx, d); err != nil {
					return "", fmt.Errorf("delete bond daily partition for date %s: %w", dateStr, err)
				}
			}
			if err := storage.StoreBondDaily(wctx, rows); err != nil {
				return "", fmt.Errorf("store bond daily for date %s: %w", dateStr, err)
			}

			emptyDays.Present(d)
			runtime.GC()
			parts = append(parts, fmt.Sprintf("STORED %d rows", len(rows)))
			return strings.Join(parts, "; "), nil
		}()
		if msg != "" {
			watch.Logf("bond_daily: > %s: %s\n", dateStr, msg)
		}
		if err != nil {
			return err
		}
	}

	if err := watch.CheckCoverage(ctx, func(ctx context.Context, from, to time.Time) ([]time.Time, error) {
		return storage.DatesWithData(ctx, "bond_daily", from, to)
	}, "bond_daily", end); err != nil {
		return fmt.Errorf("check bond_daily coverage: %w", err)
	}

	if opts.Watch {
		var tracker watch.Tracker
		return watch.RunHourly(ctx, func(ctx context.Context) error {
			now := time.Now()
			for _, d := range tracker.Days(now) {
				err := watch.ReloadDay(ctx, "watch bond_daily", d,
					func(ctx context.Context, d time.Time) ([]store.BondDaily, error) {
						return fetchBondDaily(ctx, client, d)
					},
					storage.DeleteBondDailyPartition,
					storage.StoreBondDaily,
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

// fetchBondDaily запрашивает дневные свечи всех облигаций по всем бордам за дату.
func fetchBondDaily(ctx context.Context, client *http.Client, date time.Time) ([]store.BondDaily, error) {
	dateStr := date.Format(time.DateOnly)
	var rows []store.BondDaily

	for _, board := range boards {
		boardRows, err := fetchBoardHistory(ctx, client, board, dateStr)
		if err != nil {
			return nil, fmt.Errorf("board %s: %w", board, err)
		}
		rows = append(rows, boardRows...)
	}
	return rows, nil
}

// fetchBoardHistory выкачивает все страницы history-эндпоинта для одного борда.
func fetchBoardHistory(ctx context.Context, client *http.Client, board, dateStr string) ([]store.BondDaily, error) {
	var rows []store.BondDaily
	for start := 0; ; start += 100 {
		u := fmt.Sprintf(historyURL, url.PathEscape(board)) + "?" + url.Values{
			"date":     {dateStr},
			"iss.meta": {"off"},
			"iss.json": {"extended"},
			"iss.only": {"history"},
			"start":    {fmt.Sprintf("%d", start)},
		}.Encode()

		body, err := httpjson.GetJSON(ctx, client, u)
		if err != nil {
			return nil, err
		}

		page := gjson.GetBytes(body, "1.history")
		if !page.IsArray() {
			return nil, fmt.Errorf("unexpected response: %s", httpjson.Truncate(string(body), 200))
		}

		pageRows := page.Array()
		for _, r := range pageRows {
			if r.Get("NUMTRADES").Uint() == 0 {
				continue
			}
			row, err := parseRow(r)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}

		if len(pageRows) < 100 {
			break
		}
	}
	return rows, nil
}

func parseRow(r gjson.Result) (store.BondDaily, error) {
	tradeDate, err := time.Parse("2006-01-02", r.Get("TRADEDATE").String())
	if err != nil {
		return store.BondDaily{}, fmt.Errorf("parse TRADEDATE %q: %w", r.Get("TRADEDATE").String(), err)
	}

	var duration *float64
	if v := r.Get("DURATION"); v.Exists() && v.Type != gjson.Null {
		d := v.Float()
		duration = &d
	}

	var matDate *time.Time
	if v := r.Get("MATDATE"); v.Exists() && v.Type != gjson.Null && v.String() != "" {
		if t, err := time.Parse("2006-01-02", v.String()); err == nil && t.Year() >= 1970 {
			matDate = &t
		}
	}

	return store.BondDaily{
		Time:          tradeDate,
		SecID:         r.Get("SECID").String(),
		BoardID:       r.Get("BOARDID").String(),
		Open:          r.Get("OPEN").Float(),
		High:          r.Get("HIGH").Float(),
		Low:           r.Get("LOW").Float(),
		Close:         r.Get("CLOSE").Float(),
		Value:         r.Get("VALUE").Float(),
		Volume:        r.Get("VOLUME").Uint(),
		NumTrades:     uint32(r.Get("NUMTRADES").Uint()),
		AccInt:        r.Get("ACCINT").Float(),
		YieldClose:    r.Get("YIELDCLOSE").Float(),
		YieldAtWap:    r.Get("YIELDATWAP").Float(),
		Waprice:       r.Get("WAPRICE").Float(),
		Duration:      duration,
		CouponPercent: r.Get("COUPONPERCENT").Float(),
		CouponValue:   r.Get("COUPONVALUE").Float(),
		FaceValue:     r.Get("FACEVALUE").Float(),
		FaceUnit:      r.Get("FACEUNIT").String(),
		CurrencyID:    r.Get("CURRENCYID").String(),
		MatDate:       matDate,
		BondType:      r.Get("BONDTYPE").String(),
		BondSubtype:   r.Get("BONDSUBTYPE").String(),
	}, nil
}
