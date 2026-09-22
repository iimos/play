// Package watch реализует периодическое обновление данных MOEX.
package watch

import (
	"context"
	"fmt"
	"time"

	"github.com/iimos/play/tr/tz"
)

const (
	// intradayInterval — как часто перезаливаются пятиминутные данные.
	intradayInterval = 5 * time.Minute
	// intradayOffset — люфт после границы пятиминутки, чтобы биржа успела
	// опубликовать финальные данные закрывшегося интервала.
	intradayOffset = 6 * time.Second

	// hourlyInterval — как часто перезаливаются суточные данные.
	hourlyInterval = time.Hour
	// hourlyOffset — конец часа плюс люфт (HH:59:30).
	hourlyOffset = 59*time.Minute + 30*time.Second
)

// Run5m запускает tick каждые 5 календарных минут (граница + люфт).
func Run5m(ctx context.Context, tick func(context.Context) error) error {
	return run(ctx, intradayInterval, intradayOffset, tick)
}

// RunHourly запускает tick каждый час в конце часа (плюс люфт).
func RunHourly(ctx context.Context, tick func(context.Context) error) error {
	return run(ctx, hourlyInterval, hourlyOffset, tick)
}

// run вызывает tick на каждой границе every (смещённой на offset). Работает до
// отмены ctx. Ошибки tick логируются и не прерывают цикл.
func run(ctx context.Context, every, offset time.Duration, tick func(context.Context) error) error {
	next := nextRun(time.Now(), every, offset)
	fmt.Printf("watch: следующий запуск в %s\n", next.Format(time.DateTime))

	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if err := tick(ctx); err != nil && ctx.Err() == nil {
				fmt.Printf("watch: ошибка обновления: %v\n", err)
			}
			next = nextRun(time.Now(), every, offset)
			timer.Reset(time.Until(next))
		}
	}
}

func nextRun(now time.Time, every, offset time.Duration) time.Time {
	t := now.Truncate(every).Add(offset)
	if !t.After(now) {
		t = t.Add(every)
	}
	return t
}

// maxBackfillDays ограничивает диапазон догрузки при смене/скачке даты.
const maxBackfillDays = 31

// Tracker помнит день последнего успешного тика. При смене календарного дня
// (MSK) возвращает все дни от предыдущего до текущего включительно: последняя
// свечка дня могла опубликоваться уже после полуночи, а процесс мог простоять
// несколько суток.
type Tracker struct {
	prev time.Time
}

func mskDay(t time.Time) time.Time {
	n := t.In(tz.MSK)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, tz.MSK)
}

// Days возвращает дни, которые нужно перезалить на текущем тике. Состояние не
// меняет — прогресс фиксируется вызовом Commit после успешной перезагрузки.
func (t *Tracker) Days(now time.Time) []time.Time {
	today := mskDay(now)

	start := t.prev
	if start.IsZero() {
		// Первый тик: заодно перезаливаем вчера, т.к. его финальные данные могли появиться после первичной загрузки.
		start = today.AddDate(0, 0, -1)
	}
	if today.Sub(start) > maxBackfillDays*24*time.Hour {
		start = today.AddDate(0, 0, -maxBackfillDays)
	}

	var days []time.Time
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}

// Commit фиксирует, что дни, возвращённые Days, успешно перезалиты.
func (t *Tracker) Commit(now time.Time) {
	t.prev = mskDay(now)
}

// ReloadDay перезаливает один день целиком: fetch → drop partition → store.
// Если данных нет, партицию не трогает (чтобы не потерять ранее загруженный день из-за пустого ответа API).
func ReloadDay[T any](
	ctx context.Context,
	name string,
	d time.Time,
	fetch func(context.Context, time.Time) ([]T, error),
	drop func(context.Context, time.Time) error,
	store func(context.Context, []T) error,
) error {
	dateStr := d.Format(time.DateOnly)

	data, err := fetch(ctx, d)
	if err != nil {
		return fmt.Errorf("%s %s: fetch: %w", name, dateStr, err)
	}
	if len(data) == 0 {
		fmt.Printf("%s: %s нет данных, пропускаю\n", name, dateStr)
		return nil
	}

	if err := drop(ctx, d); err != nil {
		return fmt.Errorf("%s %s: drop partition: %w", name, dateStr, err)
	}
	if err := store(ctx, data); err != nil {
		return fmt.Errorf("%s %s: store: %w", name, dateStr, err)
	}

	fmt.Printf("%s: перезалито %d записей за %s\n", name, len(data), dateStr)
	return nil
}
