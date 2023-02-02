package lib

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/nvlled/carrot"
)

const (
	CapturerIDGif = iota + 1
	CapturerIDPng
)

type Void struct{}

type Capturer interface {
	Update()
	Draw(*ebiten.Image)
	IsRunning() bool
}

func GetWindowBounds() image.Rectangle {
	x, y := ebiten.WindowPosition()
	w, h := ebiten.WindowSize()
	return image.Rect(x+3, y+3, x+w-3, y+h-3)
}

func awaitEnter(ctrl *carrot.Control) {
	ctrl.Yield()
	ctrl.YieldUntil(func() bool {
		return inpututil.IsKeyJustPressed(ebiten.KeyEnter)
	})
}

func awaitNextDraw(ctrl *carrot.Control, lasttDraw *int64) {
	prevLastDraw := *lasttDraw
	for {
		ctrl.Yield()
		if prevLastDraw < *lasttDraw {
			break
		}
	}
	ctrl.Yield()
}
