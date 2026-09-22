package supercandles

import (
	"context"
	"time"

	"github.com/iimos/play/tr/cmd/watch"
	"github.com/iimos/play/tr/moexalgo"
	"github.com/iimos/play/tr/store"
)

// watchEq каждые 5 минут целиком перезаливает текущий день по акциям.
func watchEq(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	var tracker watch.Tracker
	return watch.Run5m(ctx, func(ctx context.Context) error {
		now := time.Now()
		for _, d := range tracker.Days(now) {
			err := watch.ReloadDay(ctx, "watch eq", d,
				func(ctx context.Context, d time.Time) ([]*store.SuperCandleEq, error) {
					return fetchEqStats(ctx, sess, d)
				},
				storage.DeleteSuperEqPartition,
				storage.StoreSuperEq,
			)
			if err != nil {
				return err
			}
		}
		tracker.Commit(now)
		return nil
	})
}

// watchFO каждые 5 минут целиком перезаливает текущий день по фьючерсам.
func watchFO(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	var tracker watch.Tracker
	return watch.Run5m(ctx, func(ctx context.Context) error {
		now := time.Now()
		for _, d := range tracker.Days(now) {
			err := watch.ReloadDay(ctx, "watch fo", d,
				func(ctx context.Context, d time.Time) ([]*store.SuperCandleFO, error) {
					return fetchFOStats(ctx, sess, d)
				},
				storage.DeleteSuperFOPartition,
				storage.StoreSuperFO,
			)
			if err != nil {
				return err
			}
		}
		tracker.Commit(now)
		return nil
	})
}

// watchFX каждые 5 минут целиком перезаливает текущий день по валютам.
func watchFX(ctx context.Context, storage *store.Store, sess *moexalgo.Session) error {
	var tracker watch.Tracker
	return watch.Run5m(ctx, func(ctx context.Context) error {
		now := time.Now()
		for _, d := range tracker.Days(now) {
			err := watch.ReloadDay(ctx, "watch fx", d,
				func(ctx context.Context, d time.Time) ([]*store.SuperCandleFx, error) {
					return fetchFxStats(ctx, sess, d)
				},
				storage.DeleteSuperFxPartition,
				storage.StoreSuperFx,
			)
			if err != nil {
				return err
			}
		}
		tracker.Commit(now)
		return nil
	})
}
