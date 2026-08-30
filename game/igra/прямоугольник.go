package igra

import (
	"image"

	"golang.org/x/exp/constraints"
)

type (
	Прямоугольник image.Rectangle
)

func НовыйПрямоугольник[T constraints.Float | constraints.Integer](x0, y0, x1, y1 T) Прямоугольник {
	return Прямоугольник(image.Rect(int(x0), int(y0), int(x1), int(y1)))
}

func (п Прямоугольник) Rect() image.Rectangle {
	return image.Rectangle(п)
}

func (п *Прямоугольник) Сдвинуть(dx, dy int) {
	п.Min.X += dx
	п.Min.Y += dy
	п.Max.X += dx
	п.Max.Y += dy
}

func (п *Прямоугольник) СдвинутьВправо(смещение int) {
	п.Min.X += смещение
	п.Max.X += смещение
}

func (п *Прямоугольник) СдвинутьВлево(смещение int) {
	п.Min.X -= смещение
	п.Max.X -= смещение
}

func (п *Прямоугольник) СдвинутьВверх(смещение int) {
	п.Min.Y -= смещение
	п.Max.Y -= смещение
}

func (п *Прямоугольник) СдвинутьВниз(смещение int) {
	п.Min.Y += смещение
	п.Max.Y += смещение
}
