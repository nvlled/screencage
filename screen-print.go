package screencage

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

type ScreenPrint struct {
	currentY int32
	image    *ebiten.Image

	savedContext int64

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

func NewScreenPrint() *ScreenPrint {
	return &ScreenPrint{
		savedContext: -1,
	}
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
		y := int(scrp.currentY) + textB.Dy() + scrp.Border/2

		if textColor == nil {
			textColor = color.Black
		}

		text.Draw(scrp.image, line, font, x, y, textColor)
		scrp.currentY += int32(textB.Dy() + scrp.LineSpacing)

	}
}

func (scrp *ScreenPrint) Printf(format string, args ...any) {
	scrp.Println(fmt.Sprintf(format, args...))
}

func (scrp *ScreenPrint) PrintfAt(align byte, format string, args ...any) {
	scrp.PrintAt(align, fmt.Sprintf(format, args...))
}

func (scrp *ScreenPrint) PrintAt(align byte, str string) {
	scrp.SaveContext()
	defer scrp.RestoreContext()

	scrp.AlignX = align >> 2
	textB := text.BoundString(scrp.Font, str)
	imageB := scrp.image.Bounds()

	scrp.currentY = int32(scrp.Border / 2)
	if align&0b11 == 0b11 {
		scrp.currentY = int32(imageB.Dy()/2 - textB.Dy()/2)
	} else if align&0b01 == 0b01 {
		scrp.currentY = int32(imageB.Dy() - int(float64(textB.Dy())*2.0) - scrp.Border/2)
	}
	scrp.Println(str)
}

func (scrp *ScreenPrint) PrintColumn(left, right string) {
	alignX := scrp.AlignX
	w1 := text.BoundString(scrp.Font, left).Dx()
	w2 := text.BoundString(scrp.Font, right).Dx()

	screenW, _ := ebiten.WindowSize()
	if (w1 + w2) >= (screenW*95)/100 {
		scrp.Println(left)
		scrp.Println(right)
		return
	}

	y := scrp.currentY
	scrp.AlignX = 0b10
	scrp.Println(left)
	scrp.currentY = y
	scrp.AlignX = 0b01
	scrp.Println(right)

	scrp.AlignX = alignX
}

func (scrp *ScreenPrint) SaveContext() {
	scrp.savedContext = int64(scrp.currentY)*1000 + int64(scrp.AlignX)
}
func (scrp *ScreenPrint) RestoreContext() {
	if scrp.savedContext >= 0 {
		scrp.currentY = int32(scrp.savedContext / 1000)
		scrp.AlignX = byte(scrp.savedContext % 1000)
		scrp.savedContext = -1
	}
}
