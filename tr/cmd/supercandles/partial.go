package supercandles

import (
	"fmt"
	"time"

	"github.com/iimos/play/tr/tz"
)

// checkPartial превращает частичный ответ Algopack в ошибку.
//
// Данные дня собираются из нескольких эндпоинтов (tradestats, obstats,
// orderstats) и сливаются в один набор строк. Если один из них вернул строки,
// а другой — ничего, день был бы перезаписан неполными данными, а счётчик
// партиции (`count > 0`) потом считал бы его загруженным навсегда: дыра стала
// бы невидимой для повторного запуска. Ошибка возвращается до `DROP PARTITION`,
// поэтому прежние строки дня остаются, и следующий запуск повторит загрузку.
//
// Для текущего дня и режима latest это нормально: эндпоинты обновляются в
// разное время, и «пока пусто» — не сбой. Вчерашний день тоже допускается:
// его перезаливает watch-цикл, а на границе суток часть эндпоинтов ещё может
// отставать.
func checkPartial(date time.Time, latest bool, counts []int, names ...string) error {
	if latest {
		return nil
	}
	today := time.Now().In(tz.MSK)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, tz.MSK)
	d := date.In(tz.MSK)
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, tz.MSK)
	if !d.Before(today.AddDate(0, 0, -1)) {
		return nil
	}

	var (
		empty []string
		total int
	)
	for i, n := range counts {
		name := "endpoint"
		if i < len(names) {
			name = names[i]
		}
		if n == 0 {
			empty = append(empty, name)
		}
		total += n
	}
	if total == 0 || len(empty) == 0 {
		return nil
	}
	return fmt.Errorf("partial response for %s: no rows in %v (see --start to retry)",
		date.Format(time.DateOnly), empty)
}
