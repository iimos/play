package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/iimos/play/tr/cmd/candles"
	"github.com/iimos/play/tr/cmd/supercandles"
	"github.com/iimos/play/tr/cmd/test"
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
	_, _ = fmt.Fprintf(os.Stderr, "  load            - load all supercandles (stocks, futures, currencies)\n")
		_, _ = fmt.Fprintf(os.Stderr, "flags:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --force         - force reload all dates (delete and reload)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --start {date}  - start date (format: YYYY-MM-DD)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  --end {date}    - end date (format: YYYY-MM-DD, defaults to today)\n")
		os.Exit(1)
	}

	cmd := os.Args[1]
	ctx := context.Background()

	// Parse flags
	flags := flag.NewFlagSet(cmd, flag.ExitOnError)
	forceFlag := flags.Bool("force", false, "force reload all dates (delete and reload)")
	startFlag := flags.String("start", "", "start date (format: YYYY-MM-DD)")
	endFlag := flags.String("end", "", "end date (format: YYYY-MM-DD, defaults to today)")

	// Parse flags from os.Args[2:]
	flags.Parse(os.Args[2:])

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
		// Default to today
		endDate = time.Now()
	}

	opts := struct {
		ForceReload bool
		StartDate   time.Time
		EndDate     time.Time
	}{
		ForceReload: *forceFlag,
		StartDate:   startDate,
		EndDate:     endDate,
	}

	switch cmd {
	case "load-supereq":
		err = supercandles.LoadStocks(ctx, opts)
	case "load-superfo":
		err = supercandles.LoadFutures(ctx, opts)
	case "load-superfx":
		err = supercandles.LoadCurrencies(ctx, opts)
	case "load":
		err = supercandles.LoadAll(ctx, opts)
	case "load-candles": // deprecated
		err = candles.Load(ctx, opts)
	case "test": // for debug
		err = test.Test(ctx)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
}
