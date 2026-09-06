package futoi

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"golang.org/x/sync/errgroup"
)

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
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

	lastTableDate, err := storage.GetLastFutoiDate(ctx)
	if err != nil {
		return err
	}

	start := opts.StartDate
	end := opts.EndDate

	if start.IsZero() {
		if lastTableDate.IsZero() {
			start = time.Now().AddDate(0, 0, -10)
		} else {
			start = lastTableDate
		}
	}
	if end.IsZero() {
		end = time.Now()
	}

	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	fmt.Printf("Loading futoi data from %s to %s\n", start.Format(time.DateOnly), end.Format(time.DateOnly))

	for d := end; d.Compare(start) >= 0; d = d.AddDate(0, 0, -1) {
		fmt.Printf("> %s", d.Format(time.DateOnly))

		shouldReload := opts.ForceReload || (!lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly))

		if !shouldReload {
			count, err := storage.CountFutoiForDate(ctx, d)
			if err != nil {
				panic(err)
			}
			if count > 0 {
				fmt.Printf(": EXISTS; %d rows\n", count)
				continue
			}
		} else {
			fmt.Printf(": FORCE RELOAD")
			if !lastTableDate.IsZero() && d.Format(time.DateOnly) == lastTableDate.Format(time.DateOnly) {
				fmt.Printf(" (last date in table)")
			}
			if err := storage.DeleteFutoiPartition(ctx, d); err != nil {
				fmt.Printf(" (FAILED to delete partition: %v)\n", err)
				return fmt.Errorf("failed to delete partition for date %s: %w", d.Format(time.DateOnly), err)
			}
			fmt.Printf(" (partition deleted)")
		}

		data, err := fetchFutoi(ctx, moexSess, d)
		if err != nil {
			return err
		}
		fmt.Printf(": FETCHED %d rows\n", len(data))

		if err := storage.StoreFutoi(ctx, data); err != nil {
			return err
		}

		runtime.GC()
	}
	return nil
}

// fetchTickers возвращает список тикеров, активных на указанную дату.
// С параметром latest=1 эндпоинт отдаёт только последний снимок за день
// (~2 строки на тикер, группы FIZ/YUR), поэтому 1000 строк с запасом хватает
// на все тикеры, а полная 5-минутная история грузится уже потикерно.
func fetchTickers(ctx context.Context, sess *moexalgo.Session, dateStr string) ([]string, error) {
	set := map[string]struct{}{}

	err := moexalgo.Get(ctx, sess, "analyticalproducts/futoi/securities.json?date="+dateStr+"&latest=1", func(d *moexalgo.Futoi) {
		if d.Ticker != "" {
			set[d.Ticker] = struct{}{}
		}
	})
	if err != nil {
		return nil, err
	}

	tickers := make([]string, 0, len(set))
	for t := range set {
		tickers = append(tickers, t)
	}
	slices.Sort(tickers)
	return tickers, nil
}

func fetchFutoi(ctx context.Context, sess *moexalgo.Session, date time.Time) ([]*moexalgo.Futoi, error) {
	dateStr := date.Format(time.DateOnly)

	// На методе futoi/securities.json сломана пагинация, получить можно только первые 1000 строк
	//
	// Ответ поддержки:
	//     Привет! К сожалению, на этом эндпоинте пагинация пока не работает.
	//     Задача в работе, но по ней есть блокер. Эндпоинт можно использовать с latest=1,
	//     чтобы получить последнюю 5-минутку для всех инструментов. Но историю за день
	//     или несколько лучше брать отдельно по тикерам.
	//
	// Поэтому мы сперва получаем список всех тикеров, а потом уже по каждому отдельно вытаскиваем свечи

	tickers, err := fetchTickers(ctx, sess, dateStr)
	if err != nil {
		return nil, err
	}

	var (
		mu   sync.Mutex
		rows []*moexalgo.Futoi
	)
	gr, ctx := errgroup.WithContext(ctx)
	gr.SetLimit(10)
	for _, t := range tickers {
		t := t
		gr.Go(func() error {
			url := "analyticalproducts/futoi/securities/" + t + ".json?from=" + dateStr + "&till=" + dateStr
			err := moexalgo.Get(ctx, sess, url, func(d *moexalgo.Futoi) {
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

	slices.SortFunc(rows, func(a, b *moexalgo.Futoi) int {
		cmp := a.Time.Compare(b.Time)
		if cmp == 0 {
			cmp = strings.Compare(a.Ticker, b.Ticker)
			if cmp == 0 {
				return strings.Compare(a.ClGroup, b.ClGroup)
			}
		}
		return cmp
	})
	return rows, nil
}
