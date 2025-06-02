package main

import (
	"bytes"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/exp/constraints"
	"image"
	_ "image/png"
)

type (
	Кнопка        = ebiten.Key
	Прямоугольник = image.Rectangle
)

func НажатаКнопка(key Кнопка) bool {
	return ebiten.IsKeyPressed(key)
}

func РазмерОкна() (int, int) {
	return ebiten.WindowSize()
}

func ЗадатьПозициюОкна(x, y int) {
	ebiten.SetWindowPosition(x, y)
}

type Картинка struct {
	*ebiten.Image
}

type Опция func(*ebiten.DrawImageOptions)

func Растянуть(x, y float64) func(*ebiten.DrawImageOptions) {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Scale(x, y)
	}
}
func Переместить[T constraints.Float | constraints.Integer](x, y T) func(*ebiten.DrawImageOptions) {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Translate(float64(x), float64(y))
	}
}

func (k *Картинка) Нарисовать(k2 *Картинка, opts ...Опция) {
	o := &ebiten.DrawImageOptions{}
	for _, opt := range opts {
		opt(o)
	}
	k.DrawImage(k2.Image, o)
}

func (k *Картинка) Вырезать(rect Прямоугольник) *Картинка {
	sub := k.SubImage(rect).(*ebiten.Image)
	return &Картинка{sub}
}

func НовыйПрямоугольник[T constraints.Float | constraints.Integer](x0, y0, x1, y1 T) Прямоугольник {
	return image.Rect(int(x0), int(y0), int(x1), int(y1))
}

func НоваяКартинка(байты []byte) *Картинка {
	img, _, err := image.Decode(bytes.NewReader(байты))
	if err != nil {
		panic(err)
	}
	return &Картинка{ebiten.NewImageFromImage(img)}
}

type Игра interface {
	Обновить() error
	Нарисовать(*Картинка)
}

type gameAdapter struct {
	ОбновлениеСостояния func()
	ОбновлениеЭкрана    func(*Картинка)
}

func (g gameAdapter) Update() error {
	g.ОбновлениеСостояния()
	return nil
}

func (g gameAdapter) Draw(img *ebiten.Image) {
	ОбновлениеЭкрана(&Картинка{img})
}

func (g gameAdapter) Layout(outsideWidth, outsideHeight int) (int, int) {
	return РазмерОкна()
}

func ЗапуститьИгру(ОбновлениеСостояния func(), ОбновлениеЭкрана func(*Картинка)) error {
	g := gameAdapter{
		ОбновлениеСостояния: ОбновлениеСостояния,
		ОбновлениеЭкрана:    ОбновлениеЭкрана,
	}
	return ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{
		//ScreenTransparent: false,
	})
}
