package supercandles

import (
	"strings"
	"testing"
	"time"

	"github.com/iimos/play/tr/tz"
)

func TestCheckPartial(t *testing.T) {
	past := time.Now().In(tz.MSK).AddDate(0, 0, -3)
	today := time.Now().In(tz.MSK)
	yesterday := today.AddDate(0, 0, -1)

	// Прошедший день, все эндпоинты ответили — ок.
	if err := checkPartial(past, false, []int{10, 10, 9}, "tradestats", "obstats", "orderstats"); err != nil {
		t.Fatalf("full response must pass: %v", err)
	}

	// Один эндпоинт пуст — день перезаписывать нельзя.
	err := checkPartial(past, false, []int{10, 0, 9}, "tradestats", "obstats", "orderstats")
	if err == nil {
		t.Fatal("partial response must be an error")
	}
	if !strings.Contains(err.Error(), "obstats") {
		t.Fatalf("error should name the empty endpoint, got %v", err)
	}

	// Пусто везде — это не частичный ответ, а «нет данных» (выходной);
	// решение принимает вызывающий по len(data) == 0.
	if err := checkPartial(past, false, []int{0, 0, 0}, "a", "b", "c"); err != nil {
		t.Fatalf("all-empty must not be a partial error: %v", err)
	}

	// Текущий день обновляется по частям — не ошибка.
	if err := checkPartial(today, false, []int{10, 0, 0}, "a", "b", "c"); err != nil {
		t.Fatalf("today must be tolerant: %v", err)
	}

	// Вчерашний день тоже: на границе суток эндпоинты отстают, его перезаливает
	// watch-цикл.
	if err := checkPartial(yesterday, false, []int{10, 0, 0}, "a", "b", "c"); err != nil {
		t.Fatalf("yesterday must be tolerant: %v", err)
	}

	// latest=1 — открытая пятиминутка, тоже толерантно.
	if err := checkPartial(past, true, []int{1, 0, 0}, "a", "b", "c"); err != nil {
		t.Fatalf("latest mode must be tolerant: %v", err)
	}
}
