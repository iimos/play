package main

import (
	"context"
	"fmt"
	"github.com/iimos/play/tr/cmd/candles"
	"github.com/iimos/play/tr/cmd/supercandles"
	"github.com/iimos/play/tr/cmd/test"
	"os"
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
// https://futuresgraph.ru

func main() {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s <command>\n", os.Args[0])
		os.Exit(1)
	}

	cmd := os.Args[1]
	ctx := context.Background()
	var err error

	switch cmd {
	case "load-supereq":
		err = supercandles.LoadStocks(ctx)
	case "load-superfo":
		err = supercandles.LoadFutures(ctx)
	case "load-superfx":
		err = supercandles.LoadCurrencies(ctx)
	case "load-candles":
		err = candles.Load(ctx)
	case "test":
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
