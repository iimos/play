// Package tz предоставляет часовой пояс Московской биржи (Europe/Moscow).
package tz

import (
	"time"
	_ "time/tzdata"
)

// MSK — часовой пояс Московской биржи (Europe/Moscow).
var MSK = mustLoad("Europe/Moscow")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
