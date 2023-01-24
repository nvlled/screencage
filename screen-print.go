package main

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

type ScreenPrint struct {
	currentY int
	image    *ebiten.Image

	Color color.Color

	// 0b00 - align left
	// 0b10 - align left
	// 0b11 - align center
	// 0b01 - align right
	AlignX byte

	Font font.Face

	Border      int
	LineSpacing int
}

func (scrp *ScreenPrint) Reset(screen *ebiten.Image) {
	scrp.currentY = 0
	scrp.image = screen
}

func (scrp *ScreenPrint) Println(str string) {
	font := scrp.Font
	for _, line := range strings.Split(str, "\n") {
		if line == "" {
			line = " "
		}
		textB := text.BoundString(font, line)
		imageB := scrp.image.Bounds()

		x := scrp.Border / 2
		if scrp.AlignX&0b11 == 0b11 {
			x = imageB.Dx()/2 - textB.Dx()/2
		} else if scrp.AlignX&0b01 == 0b01 {
			x = imageB.Dx() - textB.Dx() - scrp.Border/2
		}

		textColor := scrp.Color
		if textColor == nil {
			textColor = color.Black
		}

		y := scrp.currentY + textB.Dy() + scrp.Border/2
		text.Draw(scrp.image, line, font, x, y, textColor)
		scrp.currentY += textB.Dy() + scrp.LineSpacing

		//ebitenutil.DrawRect(scrp.image, float64(x), float64(y-textB.Dy()), float64(textB.Dx()), float64(textB.Dy()), color.RGBA{255, 0, 0, 50})
	}
}

func (scrp *ScreenPrint) Printf(format string, args ...any) {
	scrp.Println(fmt.Sprintf(format, args...))
}

func (scrp *ScreenPrint) PrintAt(align byte, str string) {
	alignX := scrp.AlignX
	currentY := scrp.currentY
	defer func() {
		scrp.AlignX = alignX
		scrp.currentY = currentY
	}()

	scrp.AlignX = align >> 2
	textB := text.BoundString(scrp.Font, str)
	imageB := scrp.image.Bounds()

	scrp.currentY = scrp.Border / 2
	if align&0b11 == 0b11 {
		scrp.currentY = imageB.Dy()/2 - textB.Dy()/2
	} else if align&0b01 == 0b01 {
		scrp.currentY = imageB.Dy() - int(float64(textB.Dy())*2.0) - scrp.Border/2
	}

	scrp.Println(str)
}
