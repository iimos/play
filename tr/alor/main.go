package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/Ruvad39/go-alor"
)

// https://alor.dev - пароль по обычной схеме, но со знаком препинания

const ticker = "FEES"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	refreshToken := os.Getenv("ALOR_REFRESH_TOKEN")
	portfolio := os.Getenv("ALOR_PORTFOLIO")

	alorc := alor.NewClient(refreshToken)
	alorc.SetPortfolioID(portfolio)

	summary := must2(alorc.GetPortfolio(ctx, portfolio))
	fmt.Printf("summary: %#v\n\n", summary)

	must(orderbook(ctx, alorc, "SBER"))

	go func() {
		must(watch(ctx, alorc, portfolio))
	}()

	time.Sleep(3 * time.Second)
	buy(ctx, alorc, portfolio)

	<-ctx.Done()
}

func watch(ctx context.Context, client *alor.Client, portfolio string) error {
	// добавим коллбек на событие появление новой свечи
	client.SetOnCandle(onCandle)
	// добавим коллбек на котировки
	client.SetOnQuote(onTick)
	// добавим колл-бэк на событие появление заявки
	client.SetOnOrder(onOrder)

	// подписка на свечи
	err := client.SubscribeCandles(ctx, ticker, alor.Interval_S15, alor.WithFrequency(500))
	if err != nil {
		return err
	}
	// err = client.SubscribeCandles(ctx, "Si-6.25", alor.Interval_M1, alor.WithFrequency(500))
	// if err != nil {
	// 	return err
	// }

	// подписка на Котировки
	err = client.SubscribeQuotes(ctx, "Si-6.25", alor.WithFrequency(25))
	if err != nil {
		return err
	}

	// подпишемся на появление заявки
	err = client.SubscribeOrders(ctx, portfolio)
	if err != nil {
		return err
	}

	return nil
}

func orderbook(ctx context.Context, client *alor.Client, symbol string) error {
	orderbook, err := client.GetOrderBooks(ctx, symbol)
	if err != nil {
		return err
	}
	fmt.Printf("orderbook: %s\n\n", orderbook.String())
	bid, _ := orderbook.BestBid()
	ask, _ := orderbook.BestAsk()
	fmt.Printf("BestBid %.2f, BestAsk %.2f\n\n", bid.Price, ask.Price)
	return nil
}

func buy(ctx context.Context, client *alor.Client, portfolio string) {
	orderID, err := client.NewCreateOrderService().
		Symbol(ticker).
		Side(alor.SideTypeBuy).
		OrderType(alor.OrderTypeMarket).
		Qty(1).
		Price(0.001).
		Portfolio(portfolio).
		Comment("test").
		Do(ctx)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
	fmt.Printf("order created, OrderID=%s", orderID)
}

// сюда приходят данные по закрытым свечам
func onCandle(candle alor.Candle) {
	slog.Info("onCandle ",
		"symbol", candle.Symbol,
		"tf", candle.Interval.String(),
		"time", candle.GeTime().String(),
		"open", candle.Open,
		"high", candle.High,
		"low", candle.Low,
		"close", candle.Close,
		"volume", candle.Volume,
	)
}

func onTick(quote alor.Quote) {
	slog.Info("onTick",
		"symbol", quote.Symbol,
		"LastPriceTimestamp", quote.LastTime(),
		//"OrderBookMSTimestamp", quote.OrderBookMSTimestamp,
		"Bid", quote.Bid,
		"Ask", quote.Ask,
		"LastPrice", quote.LastPrice,
		"OpenInterest", quote.OpenInterest,
	)
}

func onOrder(order alor.Order) {
	slog.Info("OnOrder", slog.Any("order", order))
}

func must(err error) {
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
}
func must2[T any](x T, err error) T {
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
	return x
}
