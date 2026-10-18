package watch

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/iimos/play/tr/tz"
)

func TestNextRun5m(t *testing.T) {
	cases := []struct {
		now  string
		want string
	}{
		{"2026-09-17 21:28:00", "2026-09-17 21:30:06"},
		{"2026-09-17 21:30:00", "2026-09-17 21:30:06"},
		{"2026-09-17 21:30:07", "2026-09-17 21:35:06"},
	}
	for _, c := range cases {
		now, _ := time.Parse("2006-01-02 15:04:05", c.now)
		want, _ := time.Parse("2006-01-02 15:04:05", c.want)
		if got := nextRun(now, intradayInterval, intradayOffset); !got.Equal(want) {
			t.Errorf("nextRun(%s) = %s, want %s", c.now, got, want)
		}
	}
}

func TestNextRunHourly(t *testing.T) {
	cases := []struct {
		now  string
		want string
	}{
		{"2026-09-17 10:20:00", "2026-09-17 10:59:30"},
		{"2026-09-17 10:59:30", "2026-09-17 11:59:30"},
		{"2026-09-17 10:59:31", "2026-09-17 11:59:30"},
	}
	for _, c := range cases {
		now, _ := time.Parse("2006-01-02 15:04:05", c.now)
		want, _ := time.Parse("2006-01-02 15:04:05", c.want)
		if got := nextRun(now, hourlyInterval, hourlyOffset); !got.Equal(want) {
			t.Errorf("nextRun(%s) = %s, want %s", c.now, got, want)
		}
	}
}

// datesOf форматирует дни для сравнения в тестах.
func datesOf(days []time.Time) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Format(time.DateOnly)
	}
	return out
}

// eqDates сравнивает набор дней с ожидаемым списком.
func eqDates(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTrackerDays(t *testing.T) {
	var tr Tracker
	day := func(y int, m time.Month, d, h, min, s int) time.Time {
		return time.Date(y, m, d, h, min, s, 0, tz.MSK)
	}
	eq := func(got []string, want ...string) { eqDates(t, got, want...) }

	// Первый тик: вчера + сегодня, состояние не меняется до Commit.
	now := day(2026, 9, 17, 21, 30, 0)
	eq(datesOf(tr.Days(now)), "2026-09-16", "2026-09-17")
	eq(datesOf(tr.Days(now)), "2026-09-16", "2026-09-17")
	tr.Commit(now)

	// Тот же день.
	eq(datesOf(tr.Days(day(2026, 9, 17, 21, 35, 0))), "2026-09-17")

	// Смена суток: вчера + сегодня.
	tr.Commit(day(2026, 9, 17, 21, 35, 0))
	eq(datesOf(tr.Days(day(2026, 9, 18, 0, 0, 6))), "2026-09-17", "2026-09-18")

	// Простой несколько суток: все пропущенные дни.
	tr.Commit(day(2026, 9, 18, 0, 0, 6))
	eq(datesOf(tr.Days(day(2026, 9, 21, 10, 0, 6))), "2026-09-18", "2026-09-19", "2026-09-20", "2026-09-21")
}

func TestDateRange(t *testing.T) {
	days := datesOf
	eq := func(got []string, want ...string) { eqDates(t, got, want...) }
	start := time.Date(2026, 10, 15, 0, 0, 0, 0, tz.MSK)
	end := time.Date(2026, 10, 18, 0, 0, 0, 0, tz.MSK)

	eq(days(DateRange(start, end, true)), "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18")
	eq(days(DateRange(start, end, false)), "2026-10-18", "2026-10-17", "2026-10-16", "2026-10-15")

	// Один день.
	eq(days(DateRange(start, start, true)), "2026-10-15")
	eq(days(DateRange(start, start, false)), "2026-10-15")

	// start > end — пусто в любом порядке.
	eq(days(DateRange(end, start, true)))
	eq(days(DateRange(end, start, false)))

	// Разные локации и неполночь: --start парсится в UTC, end приходит из
	// time.Now().In(MSK), max(date) — в локации сервера. Набор дней должен
	// совпадать в обоих порядках и включать обе границы.
	utcStart := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	nowEnd := time.Date(2026, 10, 18, 14, 37, 0, 0, tz.MSK)
	eq(days(DateRange(utcStart, nowEnd, true)),
		"2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18")
	eq(days(DateRange(utcStart, nowEnd, false)),
		"2026-10-18", "2026-10-17", "2026-10-16", "2026-10-15")

	// Тот же день по MSK, но границы в UTC: полночь UTC — это 03:00 MSK,
	// календарный день не должен «съезжать» на предыдущий.
	eq(days(DateRange(utcStart, utcStart, true)), "2026-10-15")

	// Переход через границу суток: 21:30 UTC — это уже 16-е в MSK. Если бы
	// нормализация шла по локации аргумента, день остался бы 15-м.
	utcLate := time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC)
	eq(days(DateRange(utcLate, end, true)), "2026-10-16", "2026-10-17", "2026-10-18")
	eq(days(DateRange(utcLate, end, false)), "2026-10-18", "2026-10-17", "2026-10-16")

	// reverse(asc) == desc для любого диапазона и любых локаций.
	rev := days(DateRange(utcStart, nowEnd, false))
	slices.Reverse(rev)
	if want := days(DateRange(utcStart, nowEnd, true)); !slices.Equal(rev, want) {
		t.Fatalf("reverse(desc) = %v, want asc %v", rev, want)
	}

	// Все элементы — календарные дни MSK: лоадеры передают их в запросы
	// и в имена партиций без дополнительной нормализации.
	for _, d := range DateRange(utcStart, nowEnd, true) {
		if d.Location() != tz.MSK || d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
			t.Fatalf("day %v is not MSK midnight (%v)", d, d.Location())
		}
	}

	// Нулевое время — пусто, а не диапазон от 1 года.
	eq(days(DateRange(time.Time{}, end, true)))
	eq(days(DateRange(start, time.Time{}, false)))
}

func TestEmptyDaysReport(t *testing.T) {
	// Пусто — ничего не печатаем (проверяем, что не паникует).
	var none EmptyDays
	none.Report("x")

	mon := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, tz.MSK) }

	// Пустые дни в хвосте диапазона (типичные выходные) — не дыры: max(date)
	// на них не сдвигается, они перезапрашиваются при каждом запуске.
	var tail EmptyDays
	tail.Present(mon(16))
	tail.Add(mon(17))
	tail.Add(mon(18))
	tail.Report("tail") // ничего печатать не должен
	if got := tail.gaps(); len(got) != 0 {
		t.Fatalf("tail gaps = %v, want none", got)
	}

	// Пустой день, после которого есть день с данными, становится невидимым для
	// автостарта — о нём предупреждаем.
	var hole EmptyDays
	hole.Present(mon(15))
	hole.Add(mon(16))
	hole.Present(mon(17))
	hole.Add(mon(16)) // дубль
	got := hole.gaps()
	if len(got) != 1 || !got[0].Equal(mon(16)) {
		t.Fatalf("gaps = %v, want [2026-10-16]", got)
	}
	hole.Report("hole")

	// День, пропущенный по EXISTS, — тоже «день с данными»: пустой день перед
	// ним не должен потеряться.
	var skipped EmptyDays
	skipped.Add(mon(16))
	skipped.Present(mon(17)) // EXISTS, не запись
	if got := skipped.gaps(); len(got) != 1 || !got[0].Equal(mon(16)) {
		t.Fatalf("gaps after EXISTS = %v, want [2026-10-16]", got)
	}

	// Для источников без выходных пустые сб/вс не считаются пропуском, но
	// запоминаются отдельно — торговые субботы у MOEX бывают.
	var weekends EmptyDays
	weekends.SkipWeekends()
	weekends.Present(mon(16)) // пятница
	weekends.Add(mon(17))     // суббота
	weekends.Add(mon(18))     // воскресенье
	weekends.Present(mon(19))
	if got := weekends.gaps(); len(got) != 0 {
		t.Fatalf("weekend gaps = %v, want none", got)
	}
	if len(weekends.skipped) != 2 {
		t.Fatalf("weekend days must be remembered for the info line, got %v", weekends.skipped)
	}

	// День в UTC не должен «съезжать» на предыдущий: Add нормализует к MSK.
	var utcDay EmptyDays
	utcDay.Present(mon(16))
	utcDay.Add(time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC)) // = 16.10 00:30 MSK
	if got := utcDay.gaps(); len(got) != 0 {
		t.Fatalf("UTC day must normalize to MSK (16-е уже есть): %v", got)
	}
}

func TestCheckCoverage(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, tz.MSK) }
	to := day(18) // воскресенье

	// 12–16 — будние; 13 и 14 отсутствуют, остальное окно заполнено.
	var present []time.Time
	for d := mskDay(to).AddDate(0, 0, -CoverageDays+1); d.Before(to); d = d.AddDate(0, 0, 1) {
		if d.Day() == 13 || d.Day() == 14 {
			continue
		}
		present = append(present, d)
	}
	present = append(present, day(17), day(18))
	got := missingWeekdays(present, to)
	if len(got) != 2 || !got[0].Equal(day(13)) || !got[1].Equal(day(14)) {
		t.Fatalf("missing = %v, want 13 и 14", got)
	}

	// Всё на месте — пропусков нет; выходные не требуются.
	var all []time.Time
	for d := mskDay(to).AddDate(0, 0, -CoverageDays+1); !d.After(to); d = d.AddDate(0, 0, 1) {
		all = append(all, d)
	}
	if got := missingWeekdays(all, to); len(got) != 0 {
		t.Fatalf("missing = %v, want none", got)
	}

	// Глубже окна CoverageDays не смотрим.
	if got := missingWeekdays(nil, to); len(got) == 0 {
		t.Fatal("expected missing weekdays in empty table")
	} else if got[0].Before(mskDay(to).AddDate(0, 0, -CoverageDays)) {
		t.Fatalf("window starts too early: %v", got[0])
	}

	// Ошибка чтения из БД пробрасывается.
	wantErr := errors.New("db down")
	err := CheckCoverage(context.Background(),
		func(context.Context, time.Time, time.Time) ([]time.Time, error) { return nil, wantErr },
		"test", to)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestReloadDaySkipsEmpty(t *testing.T) {
	dropped, stored := false, false
	err := ReloadDay[int](context.Background(), "test", time.Now(),
		func(context.Context, time.Time) ([]int, error) { return nil, nil },
		func(context.Context, time.Time) error { dropped = true; return nil },
		func(context.Context, []int) error { stored = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if dropped || stored {
		t.Fatalf("empty data must not drop/store: dropped=%v stored=%v", dropped, stored)
	}
}

func TestReloadDayOrder(t *testing.T) {
	var order []string
	err := ReloadDay[int](context.Background(), "test", time.Now(),
		func(context.Context, time.Time) ([]int, error) { order = append(order, "fetch"); return []int{1}, nil },
		func(context.Context, time.Time) error { order = append(order, "drop"); return nil },
		func(context.Context, []int) error { order = append(order, "store"); return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fetch", "drop", "store"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
