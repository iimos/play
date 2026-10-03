package supercandles

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
	"github.com/iimos/play/tr/tz"
	"golang.org/x/sync/errgroup"
)

// latestInterval — период обновления снимка открытой пятиминутки в watch-режиме.
const latestInterval = 20 * time.Second

// LoadLatestEq загружает снимок открытой пятиминутки по акциям.
func LoadLatestEq(ctx context.Context, opts LoadOptions) error {
	return loadLatest(ctx, opts, storeLatestEq)
}

// LoadLatestFO загружает снимок открытой пятиминутки по фьючерсам.
func LoadLatestFO(ctx context.Context, opts LoadOptions) error {
	return loadLatest(ctx, opts, storeLatestFO)
}

// LoadLatestFx загружает снимок открытой пятиминутки по валютам.
func LoadLatestFx(ctx context.Context, opts LoadOptions) error {
	return loadLatest(ctx, opts, storeLatestFx)
}

// LoadLatestAll загружает снимки открытой пятиминутки по всем рынкам.
func LoadLatestAll(ctx context.Context, opts LoadOptions) error {
	return loadLatest(ctx, opts, storeLatestEq, storeLatestFO, storeLatestFx)
}

type latestStoreFunc func(context.Context, *store.Store, *moexalgo.Session) error

// loadLatest создаёт подключение и сессию, один раз выполняет переданные
// загрузчики и, если включён watch, повторяет их каждые latestInterval.
func loadLatest(ctx context.Context, opts LoadOptions, fns ...latestStoreFunc) error {
	storage, err := store.New()
	if err != nil {
		return err
	}
	defer storage.Close()

	sess, err := moexalgo.NewSession(moexalgo.Params{
		Token: os.Getenv("MOEX_ALGOPACK_TOKEN"),
	})
	if err != nil {
		return err
	}

	run := func(ctx context.Context) error {
		gr, ctx := errgroup.WithContext(ctx)
		for _, fn := range fns {
			fn := fn
			gr.Go(func() error { return fn(ctx, storage, sess) })
		}
		return gr.Wait()
	}

	if err := run(ctx); err != nil {
		return err
	}
	if !opts.Watch {
		return nil
	}
	return watch.RunEvery(ctx, latestInterval, run)
}

func storeLatestEq(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	candles, err := fetchEqStatsWithLatest(ctx, sess, time.Now().In(tz.MSK), true)
	if err != nil {
		return err
	}
	if len(candles) == 0 {
		fmt.Println("latest eq: нет данных, пропускаю")
		return nil
	}
	if err := storage.StoreSuperEqLatest(ctx, candles); err != nil {
		return err
	}
	fmt.Printf("latest eq: сохранено %d строк\n", len(candles))
	return nil
}

func storeLatestFO(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	candles, err := fetchFOStatsWithLatest(ctx, sess, time.Now().In(tz.MSK), true)
	if err != nil {
		return err
	}
	if len(candles) == 0 {
		fmt.Println("latest fo: нет данных, пропускаю")
		return nil
	}
	if err := storage.StoreSuperFOLatest(ctx, candles); err != nil {
		return err
	}
	fmt.Printf("latest fo: сохранено %d строк\n", len(candles))
	return nil
}

func storeLatestFx(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	candles, err := fetchFxStatsWithLatest(ctx, sess, time.Now().In(tz.MSK), true)
	if err != nil {
		return err
	}
	if len(candles) == 0 {
		fmt.Println("latest fx: нет данных, пропускаю")
		return nil
	}
	if err := storage.StoreSuperFxLatest(ctx, candles); err != nil {
		return err
	}
	fmt.Printf("latest fx: сохранено %d строк\n", len(candles))
	return nil
}
