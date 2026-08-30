package igra

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/exp/constraints"
)

type (
	Число = int
	Цвет  = color.Color
)

func РазмерОкна() (int, int) {
	return ebiten.WindowSize()
}

func ШиринаОкна() int {
	l, _ := ebiten.WindowSize()
	return l
}

func ВысотаОкна() int {
	_, h := ebiten.WindowSize()
	return h
}

func ЗадатьПозициюОкна(x, y int) {
	ebiten.SetWindowPosition(x, y)
}

type Картинка struct {
	*ebiten.Image
}

type Трансформация func(*ebiten.DrawImageOptions)

func Растянуть(x, y float64) Трансформация {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Scale(x, y)
	}
}
func Сжать(x, y float64) Трансформация {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Scale(1.0/x, 1.0/y)
	}
}
func Cместить[T constraints.Float | constraints.Integer](x, y T) Трансформация {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Translate(float64(x), float64(y))
	}
}
func Прозрачность[T constraints.Float | constraints.Integer](a T) Трансформация {
	return func(options *ebiten.DrawImageOptions) {
		options.ColorScale.ScaleAlpha(float32(a))
	}
}
func Повернуть[T constraints.Float | constraints.Integer](оборотов T) Трансформация {
	return func(options *ebiten.DrawImageOptions) {
		options.GeoM.Rotate(2 * math.Pi * float64(оборотов))
	}
}

func (k *Картинка) Нарисовать(k2 *Картинка, opts ...Трансформация) {
	o := &ebiten.DrawImageOptions{}
	for _, opt := range opts {
		opt(o)
	}
	k.DrawImage(k2.Image, o)
}

func (k *Картинка) НарисоватьТочку(x, y Число, цвет Цвет) {
	k.Set(x, y, цвет)
}

func (k *Картинка) ЦветВ(x, y Число) Цвет {
	return k.RGBA64At(x, y)
}

func (k *Картинка) Закрасить(цвет Цвет) {
	k.Fill(цвет)
}

func (k *Картинка) Границы() Прямоугольник {
	return Прямоугольник(k.Bounds())
}

func (k *Картинка) Ширина() Число {
	return k.Bounds().Dx()
}

func (k *Картинка) Высота() Число {
	return k.Bounds().Dy()
}

func (k *Картинка) Очистить() {
	k.Clear()
}

func (k *Картинка) Содержит(к2 *Картинка) bool {
	return к2.Bounds().In(k.Bounds())
}

func (k *Картинка) Написать(x Число, y Число, сообщение ...any) {
	ebitenutil.DebugPrintAt(k.Image, fmt.Sprint(сообщение...), x, y)
}

func СоздатьЦвет(красный Число, зеленый Число, синий Число) Цвет {
	return color.RGBA{uint8(красный), uint8(зеленый), uint8(синий), 255}
}

func (k *Картинка) Вырезать(rect Прямоугольник) *Картинка {
	sub := k.SubImage(rect.Rect()).(*ebiten.Image)
	return &Картинка{sub}
}

func НоваяКартинка(байты []byte) *Картинка {
	img, _, err := image.Decode(bytes.NewReader(байты))
	if err != nil {
		panic(err)
	}
	return &Картинка{ebiten.NewImageFromImage(img)}
}

func НоваяКартинкаИзФайла(путь string) *Картинка {
	file, err := os.Open(путь)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
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
	g.ОбновлениеЭкрана(&Картинка{img})
}

func (g gameAdapter) Layout(outsideWidth, outsideHeight int) (int, int) {
	return РазмерОкна()
}

func ЗапуститьИгру(ОбновлениеСостояния func(), ОбновлениеЭкрана func(*Картинка)) {
	g := gameAdapter{
		ОбновлениеСостояния: ОбновлениеСостояния,
		ОбновлениеЭкрана:    ОбновлениеЭкрана,
	}
	err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{
		//ScreenTransparent: false,
	})
	if err != nil {
		panic(err)
	}
}
