package indexsuper

import (
	"testing"
	"time"

	"github.com/iimos/play/tr/tz"
)

func msk(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, tz.MSK)
}

func TestTodayWindowEnd(t *testing.T) {
	tests := []struct {
		name         string
		now          time.Time
		sessionClose time.Time
		want         time.Time
	}{
		{
			name:         "no known close: clamp by now truncated",
			now:          msk(2026, time.September, 28, 11, 7),
			sessionClose: time.Time{},
			want:         msk(2026, time.September, 28, 11, 5),
		},
		{
			name:         "live: close later than now",
			now:          msk(2026, time.September, 28, 15, 7),
			sessionClose: msk(2026, time.September, 28, 19, 4),
			want:         msk(2026, time.September, 28, 15, 5),
		},
		{
			name:         "after close: clamp by session close, not now",
			now:          msk(2026, time.September, 28, 23, 0),
			sessionClose: msk(2026, time.September, 28, 19, 4),
			want:         msk(2026, time.September, 28, 19, 4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := todayWindowEnd(tt.now, tt.sessionClose)
			if !got.Equal(tt.want) {
				t.Fatalf("end = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBuilderTriggerCoalesce(t *testing.T) {
	b := &Builder{
		pending: map[string]time.Time{},
		wake:    make(chan struct{}, 1),
	}

	d1 := msk(2026, time.September, 28, 0, 0)
	d2 := msk(2026, time.September, 27, 0, 0)

	b.Trigger(d1)
	b.Trigger(d1)              // дубль того же дня
	b.Trigger(d1.In(time.UTC)) // тот же день в другой таймзоне
	b.Trigger(d2)

	select {
	case <-b.wake:
	default:
		t.Fatal("Trigger did not signal wake")
	}

	days := b.takePending()
	if len(days) != 2 {
		t.Fatalf("takePending len = %d, want 2 (%v)", len(days), days)
	}
	if !days[0].Equal(d2) || !days[1].Equal(d1) {
		t.Fatalf("takePending order = %v, want [%s %s]", days, d2, d1)
	}

	if got := b.takePending(); got != nil {
		t.Fatalf("second takePending = %v, want nil", got)
	}
}

func TestBuilderRequeueCap(t *testing.T) {
	b := &Builder{
		pending:  map[string]time.Time{},
		wake:     make(chan struct{}, 1),
		attempts: map[string]int{},
	}
	d := msk(2026, time.September, 28, 0, 0)

	for i := 1; i <= maxBuildAttempts; i++ {
		b.settle([]time.Time{d}, []time.Time{d})
		if got := b.takePending(); len(got) != 1 {
			t.Fatalf("attempt %d: requeued day missing", i)
		}
	}

	// Попытка сверх лимита: день больше не ставится в очередь.
	b.settle([]time.Time{d}, []time.Time{d})
	if got := b.takePending(); len(got) != 0 {
		t.Fatalf("day requeued past cap: %v", got)
	}
}

func TestBuilderSettleResetsAttempts(t *testing.T) {
	b := &Builder{
		pending:  map[string]time.Time{},
		wake:     make(chan struct{}, 1),
		attempts: map[string]int{},
	}
	d := msk(2026, time.September, 28, 0, 0)
	key := d.Format(time.DateOnly)

	b.settle([]time.Time{d}, []time.Time{d}) // неудача
	if b.attempts[key] != 1 {
		t.Fatalf("attempts = %d, want 1", b.attempts[key])
	}
	b.settle([]time.Time{d}, nil) // успех сбрасывает счётчик
	if _, ok := b.attempts[key]; ok {
		t.Fatalf("attempts not reset after success: %v", b.attempts)
	}
	b.settle([]time.Time{d}, []time.Time{d}) // снова неудача — счёт с нуля
	if b.attempts[key] != 1 {
		t.Fatalf("attempts = %d, want 1 after reset", b.attempts[key])
	}
}
