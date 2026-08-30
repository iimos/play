package igra

import "time"

var (
	Секунда      = time.Second
	Миллисекунда = time.Millisecond
)

func ТекущееВремя() time.Time {
	return time.Now()
}

func ТекущееВремяСек() int64 {
	return time.Now().Unix()
}

func ТекущееВремяМсек() int64 {
	return time.Now().UnixMilli()
}

func Спать(количество int, время time.Duration) {
	time.Sleep(время * time.Duration(количество))
}
