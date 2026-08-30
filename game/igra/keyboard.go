package igra

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type (
	Кнопка = ebiten.Key
)

func НажатаКнопка(key Кнопка) bool {
	return ebiten.IsKeyPressed(key)
}

const (
	КнопкаВверх  Кнопка = ebiten.KeyUp
	КнопкаВниз   Кнопка = ebiten.KeyDown
	КнопкаВлево  Кнопка = ebiten.KeyLeft
	КнопкаВправо Кнопка = ebiten.KeyRight
	КнопкаПробел Кнопка = ebiten.KeySpace
	КнопкаЭнтер  Кнопка = ebiten.KeyEnter
)
