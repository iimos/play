// Package watch реализует периодическое обновление данных MOEX.
package watch

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
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

// RunEvery запускает tick каждые every без привязки к календарным границам
// (offset = 0). Используется для частого обновления открытой пятиминутки.
func RunEvery(ctx context.Context, every time.Duration, tick func(context.Context) error) error {
	return run(ctx, every, 0, tick)
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

// DateRange возвращает дни от start до end включительно. При asc == true —
// по возрастанию, иначе по убыванию; при start > end срез пуст.
//
// Границы нормализуются к календарным дням MSK: start и end приходят из разных
// источников (--start парсится в UTC, --end/time.Now() — в MSK, max(date) из БД
// — в локации сервера), и без нормализации «тот же диапазон» содержит разные
// дни в зависимости от времени суток и локации: например
// start=2026-10-15 00:00 UTC, end=2026-10-18 00:00 MSK даёт по возрастанию
// 15–17 (теряется последний день), по убыванию 16–18 (теряется первый).
//
// Порядок важен для самовосстановления после обрыва: точка возобновления
// автостарта выводится из max(date) таблицы, поэтому обрыв дозагрузки вниз
// («справа налево») оставляет последнюю дату в конце диапазона и повторный
// запуск без флагов дыру уже не видит. Вверх («слева направо») последняя
// записанная дата остаётся перед дырой, и ретрай продолжает ровно с неё.
func DateRange(start, end time.Time, asc bool) []time.Time {
	if start.IsZero() || end.IsZero() {
		return nil
	}
	start, end = mskDay(start), mskDay(end)
	if start.After(end) {
		return nil
	}
	// Подсказка ёмкости: в MSK переходов на летнее время нет c 2014, на более
	// ранних диапазонах дней может оказаться на 1 больше — слайс просто вырастет.
	n := int(end.Sub(start).Hours()/24) + 1
	days := make([]time.Time, 0, n)
	if asc {
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			days = append(days, d)
		}
		return days
	}
	for d := end; !d.Before(start); d = d.AddDate(0, 0, -1) {
		days = append(days, d)
	}
	return days
}

// maxBackfillDays ограничивает диапазон догрузки при смене/скачке даты.
const maxBackfillDays = 31

// logMu сериализует вывод: в команде `load` лоадеры работают параллельно, и
// строка, собранная из нескольких fmt.Printf, склеивается с чужой.
var logMu sync.Mutex

// Logf печатает строку целиком, не перемешивая её с выводом других лоадеров.
func Logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Printf(format, args...)
}

// EmptyDays собирает дни, за которые источник вернул пустой ответ, а после них
// был день с данными.
//
// Нужно для предупреждения в конце прогона: при автостарте точка возобновления
// выводится из max(date), поэтому пустой торговый день (источник ещё не
// опубликовал данные или вернул 200 с пустым массивом) после записи следующих
// дней перестаёт быть видимым — повторный запуск без флагов начнёт с max(date)
// и этот день уже не пересмотрит. Пустые дни в хвосте диапазона не считаются:
// max(date) на них не сдвигается, и они перезапрашиваются при каждом запуске.
type EmptyDays struct {
	days []time.Time
	// lastPresent — последний день, за который в таблице есть данные:
	// записанные сейчас или уже лежавшие.
	lastPresent time.Time
	// skipWeekend включается для источников, у которых в сб/вс данных обычно не
	// бывает. Пропущенные выходные не теряются: они печатаются отдельной
	// строкой, чтобы объявленная торговая суббота (они бывают: 2024-04-27,
	// 2025-11-01, ...) не осталась незамеченной.
	skipWeekend bool
	skipped     []time.Time
}

// SkipWeekends отмечает, что источник обычно не публикует данные по субботам и
// воскресеньям: такие пустые дни не попадают в отчёт о дырах, но печатаются
// отдельной информационной строкой — торговые субботы у MOEX бывают.
func (e *EmptyDays) SkipWeekends() {
	e.skipWeekend = true
}

// Add запоминает день с пустым ответом. Вызывать только при автостарте: при
// явном --start диапазон задан вручную.
func (e *EmptyDays) Add(d time.Time) {
	d = mskDay(d)
	if e.skipWeekend {
		switch d.Weekday() {
		case time.Saturday, time.Sunday:
			e.skipped = append(e.skipped, d)
			return
		}
	}
	e.days = append(e.days, d)
}

// Present отмечает день, за который в таблице есть данные — записанные сейчас
// или уже лежавшие (пропуск по EXISTS). Нужен, чтобы пустой день перед таким
// днём не потерялся: автостарт до него уже не дотянется.
func (e *EmptyDays) Present(d time.Time) {
	d = mskDay(d)
	if d.After(e.lastPresent) {
		e.lastPresent = d
	}
}

// gaps возвращает пустые дни, до которых автостарт уже не дотянется: за ними
// есть день с данными. Уникальные, по возрастанию.
func (e *EmptyDays) gaps() []time.Time {
	seen := map[string]time.Time{}
	for _, d := range e.days {
		if d.Before(e.lastPresent) {
			seen[d.Format(time.DateOnly)] = d
		}
	}
	gaps := make([]time.Time, 0, len(seen))
	for _, d := range seen {
		gaps = append(gaps, d)
	}
	slices.SortFunc(gaps, func(a, b time.Time) int { return a.Compare(b) })
	return gaps
}

// Report печатает предупреждение о пустых днях, за которыми были дни с данными
// (то есть о дырах, до которых автостарт уже не дотянется). what — имя источника
// для лога.
func (e *EmptyDays) Report(what string) {
	if e.skipWeekend {
		if days := dayList(e.skipped, 10); days != "" {
			Logf("info: %s: в выходные источник пуст (%s); если это объявленная "+
				"торговая суббота — нужен --start\n", what, days)
		}
	}
	gaps := e.gaps()
	if len(gaps) == 0 {
		return
	}
	Logf("warning: %s: пустой ответ источника в середине диапазона за %d дн. (%s); "+
		"если это не праздник, автостарт эти дни не закроет — догрузка: --start=%s\n",
		what, len(gaps), dayList(gaps, 10), gaps[0].Format(time.DateOnly))
}

// dayList форматирует дни через запятую, обрезая длинный список.
func dayList(days []time.Time, shownMax int) string {
	if len(days) == 0 {
		return ""
	}
	uniq := map[string]time.Time{}
	for _, d := range days {
		uniq[d.Format(time.DateOnly)] = d
	}
	sorted := make([]time.Time, 0, len(uniq))
	for _, d := range uniq {
		sorted = append(sorted, d)
	}
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })

	shown, extra := sorted, 0
	if len(shown) > shownMax {
		extra, shown = len(shown)-shownMax, shown[:shownMax]
	}
	parts := make([]string, len(shown))
	for i, d := range shown {
		parts[i] = d.Format(time.DateOnly)
	}
	res := strings.Join(parts, ", ")
	if extra > 0 {
		res += fmt.Sprintf(" и ещё %d", extra)
	}
	return res
}

// CoverageDays — сколько последних дней проверяется на пропуски в конце
// прогона. Такая проверка нужна потому, что автостарт выводит точку
// возобновления из max(date) и дыры ДО неё не видит вообще: без неё прогон
// печатает успех, хотя торговых дней в таблице может не быть.
const CoverageDays = 30

// CheckCoverage сверяет будние дни окна [to-CoverageDays, to] с фактическим
// наличием строк в таблице и печатает пропуски. Проверка только читает БД и
// ничего не перезаписывает.
//
// Будние дни в праздники тоже попадут в отчёт — это осознанно: отличить
// праздник от недогрузки без торгового календаря нельзя, а молчание дороже
// лишней строки в логе.
func CheckCoverage(ctx context.Context, dates func(context.Context, time.Time, time.Time) ([]time.Time, error), what string, to time.Time) error {
	from := mskDay(to).AddDate(0, 0, -CoverageDays+1)
	present, err := dates(ctx, from, to)
	if err != nil {
		return err
	}
	missing := missingWeekdays(present, to)
	if len(missing) == 0 {
		return nil
	}
	Logf("warning: %s: нет данных за %d будних дн. (%s); автостарт их не видит — "+
		"догрузка: --start=%s\n",
		what, len(missing), dayList(missing, 10), missing[0].Format(time.DateOnly))
	return nil
}

// missingWeekdays возвращает будние дни из окна CoverageDays, за которые нет
// ни одной даты в present. Сб/вс не проверяются.
func missingWeekdays(present []time.Time, to time.Time) []time.Time {
	seen := make(map[string]struct{}, len(present))
	for _, d := range present {
		seen[mskDay(d).Format(time.DateOnly)] = struct{}{}
	}
	to = mskDay(to)
	from := to.AddDate(0, 0, -CoverageDays+1)

	var missing []time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		switch d.Weekday() {
		case time.Saturday, time.Sunday:
			continue
		}
		if _, ok := seen[d.Format(time.DateOnly)]; !ok {
			missing = append(missing, d)
		}
	}
	return missing
}

// DayRange возвращает DateRange и предупреждает, если диапазон пуст: иначе
// опечатка в --start/--end (или дата из будущего в таблице) выглядит как
// успешный прогон, который ничего не сделал. autoStart — старт выведен из
// таблицы, а не задан пользователем: тогда предупреждение адресует --end.
func DayRange(start, end time.Time, asc, autoStart bool) []time.Time {
	days := DateRange(start, end, asc)
	if len(days) > 0 || start.IsZero() {
		return days
	}
	s := mskDay(start).Format(time.DateOnly)
	if end.IsZero() {
		Logf("warning: пустой диапазон дат: --start %s, --end не задан\n", s)
		return days
	}
	e := mskDay(end).Format(time.DateOnly)
	if autoStart {
		Logf("warning: пустой диапазон дат: --end %s раньше точки возобновления %s; "+
			"ничего не загружено, укажите --start\n", e, s)
	} else {
		Logf("warning: пустой диапазон дат: --start %s > --end %s\n", s, e)
	}
	return days
}

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

	// Отмену контекста уважаем при скачивании, но не между DROP PARTITION и
	// INSERT: иначе Ctrl-C в этом окне оставит день пустым.
	wctx := context.WithoutCancel(ctx)
	if err := drop(wctx, d); err != nil {
		return fmt.Errorf("%s %s: drop partition: %w", name, dateStr, err)
	}
	if err := store(wctx, data); err != nil {
		return fmt.Errorf("%s %s: store: %w", name, dateStr, err)
	}

	fmt.Printf("%s: перезалито %d записей за %s\n", name, len(data), dateStr)
	return nil
}
