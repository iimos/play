package supercandles

import "time"

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	// Watch включает режим постоянного обновления: после первичной загрузки
	// каждые 5 минут целиком перезаливается текущий день.
	Watch bool
}
