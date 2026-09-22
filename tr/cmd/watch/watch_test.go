package watch

import (
	"context"
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
