package candles

import "time"

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
}