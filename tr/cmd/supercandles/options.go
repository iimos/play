package supercandles

import "time"

type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	// Watch включает режим постоянного обновления: после первичной загрузки
	// каждые 5 минут целиком перезаливается текущий день.
	Watch bool
	// AfterReload вызывается после успешной перезагрузки дня в watch-режиме
	// (например, чтобы пересобрать производные данные). Не должен блокировать.
	AfterReload func(day time.Time)
}
