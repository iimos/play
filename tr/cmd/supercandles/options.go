package supercandles

import "time"

// LoadOptions — общие параметры загрузки суперсвечей.
//
// Замечание: команды load-*-latest (снимок открытой пятиминутки) используют
// только поле Watch; StartDate/EndDate/ForceReload для них не применяются,
// так как API latest=1 всегда отдаёт текущий интервал.
type LoadOptions struct {
	ForceReload bool
	StartDate   time.Time
	EndDate     time.Time
	// Watch включает режим постоянного обновления. Для исторических загрузчиков
	// (LoadStocks/LoadFutures/LoadCurrencies) каждые 5 минут целиком
	// перезаливается текущий день; для latest-загрузчиков снимок открытой
	// пятиминутки обновляется каждые ~20 секунд.
	Watch bool
	// AfterReload вызывается после успешной перезагрузки дня в watch-режиме
	// (например, чтобы пересобрать производные данные). Не должен блокировать.
	AfterReload func(day time.Time)
}
