package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iimos/play/tr/cmd/bonddaily"
	"github.com/iimos/play/tr/cmd/candles"
	"github.com/iimos/play/tr/cmd/futoi"
	"github.com/iimos/play/tr/cmd/indexdata"
	"github.com/iimos/play/tr/cmd/indexsuper"
	"github.com/iimos/play/tr/cmd/openpositions"
	"github.com/iimos/play/tr/cmd/securities"
	"github.com/iimos/play/tr/cmd/supercandles"
	"github.com/iimos/play/tr/cmd/test"
	"github.com/iimos/play/tr/tz"
	"golang.org/x/sync/errgroup"
)

// https://iss.moex.com/iss/reference/
// https://iss.moex.com/iss/engines/stock/markets/shares/securities/YDEX/candles.json?from=2024-08-01&till=2024-08-10&interval=60
// https://iss.moex.com/iss/engines/stock/markets/index/securities/IMOEX/candles.json?from=2024-10-04&till=2024-10-04&interval=60
// https://iss.moex.com/iss/securities.json?q=ГАЗП

// https://moexalgo.github.io/des/supercandles/
// https://moexalgo.github.io/api/rest/
// https://iss.moex.com/iss/datashop/algopack/eq/tradestats/ROSN?date=2024-09-02
// https://iss.moex.com/iss/datashop/algopack/eq/orderstats/?date=2024-10-02
// https://iss.moex.com/iss/datashop/algopack/eq/obstats?date=2024-09-02
// https://www.moex.com/algopackvisual/supercandles?ticker=GAZP - UI https://teletype.in/@timredz/megaalerts

func main() {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s <command> [flags]\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "commands:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-supereq    - load stock supercandles\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-superfo    - load futures supercandles\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-superfx    - load currency supercandles\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-supereq-latest - save snapshot of the open 5-min supercandle (stocks)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-superfo-latest - save snapshot of the open 5-min supercandle (futures)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-superfx-latest - save snapshot of the open 5-min supercandle (currency)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-latest     - save snapshots of the open 5-min supercandles (all markets)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-futoi      - load futures open interest (FUTOI)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-iss-openpositions - load daily futures open positions by phys/legal (ISS statistics)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load            - load all (supercandles, futoi, openpositions, bond daily) then rebuild index supercandles\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-securities - load securities metadata (names, emitents, etc.)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-bond-daily    - load daily bond candles with bond attributes\n")
		_, _ = fmt.Fprintf(os.Stderr, "  load-index      - load index candles and constituent weights (all available indices)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  build-index-super  - synthesize index supercandles from stock supercandles and weights\n")
		_, _ = fmt.Fprintf(os.Stderr, "flags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --force         - force reload all dates (delete and reload)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --start {date}  - start date (format: YYYY-MM-DD, defaults to last date in table)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --end {date}    - end date (format: YYYY-MM-DD, defaults to today)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --index {ids}   - comma-separated index ids (build-index-super only)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --watch         - keep running: reload the current day every 5 minutes\n")
		_, _ = fmt.Fprintf(os.Stderr, "                    (daily data every hour; latest supercandles every 20s)\n")
		_, _ = fmt.Fprintf(os.Stderr, "                    incompatible with --start/--end\n")
		os.Exit(1)
	}

	cmd := os.Args[1]
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// После первого сигнала возвращаем дефолтную обработку, чтобы повторный
	// Ctrl-C мог принудительно завершить процесс.
	go func() {
		<-ctx.Done()
		stop()
	}()

	// Parse flags
	flags := flag.NewFlagSet(cmd, flag.ExitOnError)
	forceFlag := flags.Bool("force", false, "force reload all dates (delete and reload)")
	startFlag := flags.String("start", "", "start date (format: YYYY-MM-DD, defaults to last date in table)")
	endFlag := flags.String("end", "", "end date (format: YYYY-MM-DD, defaults to today)")
	watchFlag := flags.Bool("watch", false, "keep running and reload the current day periodically")
	indexFlag := flags.String("index", "", "comma-separated index ids (build-index-super only)")

	// Parse flags from os.Args[2:]
	flags.Parse(os.Args[2:])

	if *watchFlag && (*startFlag != "" || *endFlag != "") {
		_, _ = fmt.Fprintln(os.Stderr, "--watch is not compatible with --start/--end")
		os.Exit(1)
	}

	if *watchFlag && !watchSupported(cmd) {
		_, _ = fmt.Fprintf(os.Stderr, "--watch is not supported for command %s\n", cmd)
		os.Exit(1)
	}

	// Parse dates
	var startDate, endDate time.Time
	var err error

	if *startFlag != "" {
		startDate, err = time.Parse(time.DateOnly, *startFlag)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "invalid start date format: %s (expected: YYYY-MM-DD)\n", *startFlag)
			os.Exit(1)
		}
	}

	if *endFlag != "" {
		endDate, err = time.Parse(time.DateOnly, *endFlag)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "invalid end date format: %s (expected: YYYY-MM-DD)\n", *endFlag)
			os.Exit(1)
		}
	} else {
		// Default to today (Moscow)
		endDate = time.Now().In(tz.MSK)
	}

	opts := struct {
		ForceReload bool
		StartDate   time.Time
		EndDate     time.Time
		Watch       bool
	}{
		ForceReload: *forceFlag,
		StartDate:   startDate,
		EndDate:     endDate,
		Watch:       *watchFlag,
	}

	indices := parseIndices(*indexFlag)
	superOpts := supercandles.LoadOptions{
		ForceReload: opts.ForceReload,
		StartDate:   opts.StartDate,
		EndDate:     opts.EndDate,
		Watch:       opts.Watch,
	}
	indexOpts := indexdata.LoadOptions{
		ForceReload: opts.ForceReload,
		StartDate:   opts.StartDate,
		EndDate:     opts.EndDate,
		Watch:       opts.Watch,
	}
	superIndexOpts := indexsuper.LoadOptions{
		ForceReload: opts.ForceReload,
		StartDate:   opts.StartDate,
		EndDate:     opts.EndDate,
		Watch:       opts.Watch,
		Indices:     indices,
	}

	switch cmd {
	case "load-supereq":
		err = supercandles.LoadStocks(ctx, superOpts)
	case "load-superfo":
		err = supercandles.LoadFutures(ctx, superOpts)
	case "load-superfx":
		err = supercandles.LoadCurrencies(ctx, superOpts)
	case "load-supereq-latest":
		err = supercandles.LoadLatestEq(ctx, superOpts)
	case "load-superfo-latest":
		err = supercandles.LoadLatestFO(ctx, superOpts)
	case "load-superfx-latest":
		err = supercandles.LoadLatestFx(ctx, superOpts)
	case "load-latest":
		err = supercandles.LoadLatestAll(ctx, superOpts)
	case "load-futoi":
		err = futoi.Load(ctx, opts)
	case "load-iss-openpositions":
		err = openpositions.Load(ctx, opts)
	case "load-index":
		if *indexFlag != "" {
			_, _ = fmt.Fprintln(os.Stderr, "warning: --index is ignored for load-index: the whole day is reloaded for all available indices")
		}
		err = indexdata.Load(ctx, indexOpts)
	case "build-index-super":
		err = indexsuper.Build(ctx, superIndexOpts)
	case "load":
		// В watch-режиме синтез super_index запускается по хукам лоадеров
		// (debounced Builder). --index для load игнорируется: собираются все индексы.
		var builder *indexsuper.Builder
		if opts.Watch {
			b, berr := indexsuper.NewBuilder(ctx, nil)
			if berr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: super_index build disabled: %v\n", berr)
			} else {
				builder = b
				defer builder.Close()
				superOpts.AfterReload = builder.Trigger
				indexOpts.AfterReload = builder.Trigger
			}
		}

		gr, grctx := errgroup.WithContext(ctx)
		if builder != nil {
			gr.Go(func() error { return builder.Run(grctx) })
		}
		if opts.Watch {
			// Держим снимки открытой пятиминутки в актуальном состоянии.
			gr.Go(func() error { return supercandles.LoadLatestAll(grctx, superOpts) })
		}
		gr.Go(func() error { return supercandles.LoadAll(grctx, superOpts) })
		gr.Go(func() error { return futoi.Load(grctx, opts) })
		gr.Go(func() error { return openpositions.Load(grctx, opts) })
		gr.Go(func() error { return bonddaily.Load(grctx, opts) })
		gr.Go(func() error { return indexdata.Load(grctx, indexOpts) })
		err = gr.Wait()

		// Без watch источники уже загружены — собираем историю один раз.
		if err == nil && !opts.Watch {
			err = indexsuper.Build(ctx, indexsuper.LoadOptions{
				ForceReload: opts.ForceReload,
				StartDate:   opts.StartDate,
				EndDate:     opts.EndDate,
			})
		}
	case "load-candles": // deprecated
		err = candles.Load(ctx, opts)
	case "load-bond-daily":
		err = bonddaily.Load(ctx, opts)
	case "load-securities":
		err = securities.Load(ctx)
	case "test": // for debug
		err = test.Test(ctx)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}

	// При отмене (Ctrl-C/SIGTERM) выходим штатно: ошибка могла прийти в виде
	// cause контекста (напр. "interrupt signal received"), а не context.Canceled.
	if err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
}

// watchSupported сообщает, поддерживает ли команда режим --watch.
func watchSupported(cmd string) bool {
	switch cmd {
	case "load", "load-supereq", "load-superfo", "load-superfx", "load-futoi",
		"load-iss-openpositions", "load-bond-daily", "load-index", "build-index-super",
		"load-latest", "load-supereq-latest", "load-superfo-latest", "load-superfx-latest":
		return true
	}
	return false
}

// parseIndices разбирает список кодов индексов из флага --index.
func parseIndices(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var res []string
	for _, part := range strings.Split(s, ",") {
		part = strings.ToUpper(strings.TrimSpace(part))
		if part != "" {
			res = append(res, part)
		}
	}
	return res
}
