package supercandles

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

func LoadAll(ctx context.Context, opts LoadOptions) error {
	fmt.Println("Starting combined data loading...")

	gr, ctx := errgroup.WithContext(ctx)

	gr.Go(func() error {
		fmt.Println("Loading stocks (load-supereq)...")
		return LoadStocks(ctx, opts)
	})

	gr.Go(func() error {
		fmt.Println("Loading futures (load-superfo)...")
		return LoadFutures(ctx, opts)
	})

	gr.Go(func() error {
		fmt.Println("Loading currencies (load-superfx)...")
		return LoadCurrencies(ctx, opts)
	})

	if err := gr.Wait(); err != nil {
		return fmt.Errorf("error during combined loading: %w", err)
	}

	fmt.Println("All data loading completed successfully!")
	return nil
}