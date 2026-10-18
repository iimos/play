package watch

import (
	"context"
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

func TestTrackerDays(t *testing.T) {
	var tr Tracker
	day := func(y int, m time.Month, d, h, min, s int) time.Time {
		return time.Date(y, m, d, h, min, s, 0, tz.MSK)
	}
	dates := func(days []time.Time) []string {
		out := make([]string, len(days))
		for i, d := range days {
			out[i] = d.Format(time.DateOnly)
		}
		return out
	}
	eq := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}

	// Первый тик: вчера + сегодня, состояние не меняется до Commit.
	now := day(2026, 9, 17, 21, 30, 0)
	eq(dates(tr.Days(now)), "2026-09-16", "2026-09-17")
	eq(dates(tr.Days(now)), "2026-09-16", "2026-09-17")
	tr.Commit(now)

	// Тот же день.
	eq(dates(tr.Days(day(2026, 9, 17, 21, 35, 0))), "2026-09-17")

	// Смена суток: вчера + сегодня.
	tr.Commit(day(2026, 9, 17, 21, 35, 0))
	eq(dates(tr.Days(day(2026, 9, 18, 0, 0, 6))), "2026-09-17", "2026-09-18")

	// Простой несколько суток: все пропущенные дни.
	tr.Commit(day(2026, 9, 18, 0, 0, 6))
	eq(dates(tr.Days(day(2026, 9, 21, 10, 0, 6))), "2026-09-18", "2026-09-19", "2026-09-20", "2026-09-21")
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

func TestDateRange(t *testing.T) {
	start := time.Date(2026, 10, 15, 0, 0, 0, 0, tz.MSK)
	end := time.Date(2026, 10, 18, 0, 0, 0, 0, tz.MSK)

	eqDates(t, datesOf(DateRange(start, end, true)),
		"2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18")
	eqDates(t, datesOf(DateRange(start, end, false)),
		"2026-10-18", "2026-10-17", "2026-10-16", "2026-10-15")

	// Один день.
	eqDates(t, datesOf(DateRange(start, start, true)), "2026-10-15")
	eqDates(t, datesOf(DateRange(start, start, false)), "2026-10-15")

	// start > end — пусто в любом порядке.
	eqDates(t, datesOf(DateRange(end, start, true)))
	eqDates(t, datesOf(DateRange(end, start, false)))

	// Разные локации и неполночь: --start парсится в UTC, end приходит из
	// time.Now().In(MSK), max(date) — в локации сервера. Набор дней должен
	// совпадать в обоих порядках и включать обе границы.
	utcStart := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	nowEnd := time.Date(2026, 10, 18, 14, 37, 0, 0, tz.MSK)
	eqDates(t, datesOf(DateRange(utcStart, nowEnd, true)),
		"2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18")
	eqDates(t, datesOf(DateRange(utcStart, nowEnd, false)),
		"2026-10-18", "2026-10-17", "2026-10-16", "2026-10-15")

	// Тот же день по MSK, но границы в UTC: полночь UTC — это 03:00 MSK,
	// календарный день не должен «съезжать» на предыдущий.
	eqDates(t, datesOf(DateRange(utcStart, utcStart, true)), "2026-10-15")

	// Переход через границу суток: 21:30 UTC — это уже 16-е в MSK. Если бы
	// нормализация шла по локации аргумента, день остался бы 15-м.
	utcLate := time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC)
	eqDates(t, datesOf(DateRange(utcLate, end, true)), "2026-10-16", "2026-10-17", "2026-10-18")
	eqDates(t, datesOf(DateRange(utcLate, end, false)), "2026-10-18", "2026-10-17", "2026-10-16")

	// reverse(asc) == desc для любого диапазона и любых локаций.
	rev := datesOf(DateRange(utcStart, nowEnd, false))
	slices.Reverse(rev)
	if want := datesOf(DateRange(utcStart, nowEnd, true)); !slices.Equal(rev, want) {
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
	eqDates(t, datesOf(DateRange(time.Time{}, end, true)))
	eqDates(t, datesOf(DateRange(start, time.Time{}, false)))
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
