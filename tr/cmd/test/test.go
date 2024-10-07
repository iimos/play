package test

import (
	"context"
	"fmt"
	"github.com/WLM1ke/gomoex"
	"net/http"
	"time"
)

func Test(ctx context.Context) error {
	iss := gomoex.NewISSClient(&http.Client{Timeout: 10 * time.Second})

	//candles := must(iss.MarketCandles(ctx, gomoex.EngineStock, gomoex.MarketShares, "ROSN", "2024-10-01", "2024-10-01", gomoex.IntervalMin1))
	candles := must(iss.MarketCandles(ctx, gomoex.EngineStock, gomoex.MarketIndex, "IMOEX2", "2024-10-01", "2024-10-01", gomoex.IntervalMin1))
	//candles := must(iss.MarketCandles(ctx, gomoex.EngineStock, gomoex.MarketShares, "LQDT", "2024-10-01", "2024-10-01", gomoex.IntervalMin1))

	fmt.Printf("len(candles) = %d\n", len(candles))
	fmt.Printf("candles[0]: %+v\n", candles[0])

	return nil
}

func must[T any](x T, err error) T {
	if err != nil {
		panic(err)
	}
	return x
}
