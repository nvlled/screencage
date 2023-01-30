package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync/atomic"
	"time"

	gif "github.com/nvlled/gogif"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/kbinani/screenshot"
	"github.com/nvlled/carrot"
)

const (
	GifCapturerStateStopped int32 = iota
	GifCapturerStateRunning
	GifCapturerStateSaving
	GifCapturerStateDisposed
)

const (
	GifCapturerActionScreenshot = iota
	GifCapturerActionSave
)

type GifCapturer struct {
	saveFilename string

	numImages    int
	numProcessed int

	running atomic.Bool

	draw func(*ebiten.Image)

	script *carrot.Script

	game *Game
	scrp *ScreenPrint

	Error error
}

func NewGifCapturer(game *Game) *GifCapturer {
	capturer := &GifCapturer{
		game: game,
		scrp: game.scrp,
	}
	capturer.script = carrot.Start(capturer.coroutine)
	return capturer
}

type GifFrame struct {
	Image   *image.RGBA
	CsDelay int
}

func SaveOneGif(encoder *gif.StreamEncoder, img *image.RGBA, delay int) *Task[Void] {
	task := &Task[Void]{}
	go func() {
		defer task.Finish()

		quantizer := quantize.MedianCutQuantizer{}
		emptyPalette := make([]color.Color, 0, 256)
		pal := quantizer.Quantize(emptyPalette, img)

		palleted := image.NewPaletted(img.Rect, pal)
		draw.Src.Draw(palleted, img.Bounds(), img, image.Point{})

		encoder.Encode(palleted, delay, gif.DisposalNone)
	}()

	return task
}

func (capturer *GifCapturer) coroutine(ctrl *carrot.Control) {
	awaitEnter := func() {
		ctrl.Yield()
		ctrl.YieldUntil(func() bool {
			return inpututil.IsKeyJustPressed(ebiten.KeyEnter)
		})
	}

	goto START

ERROR:
	println("error", capturer.Error.Error())
	capturer.draw = capturer.drawError
	awaitEnter()
	capturer.Error = nil

	goto START

START:
	// inactive
	println("* inactive")
	capturer.game.borderLight = ColorTeal
	capturer.game.borderDark = ColorTealDark
	capturer.running.Store(false)
	capturer.draw = capturer.drawInactive
	capturer.numImages = 0
	capturer.numProcessed = 0
	awaitEnter()

	capturer.saveFilename = capturer.game.getNextOutFilename()
	file, err := os.OpenFile(capturer.saveFilename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		capturer.Error = err
		goto ERROR
	}
	encoder := gif.NewStreamEncoder(file, &gif.StreamEncoderOptions{})

	// active
	var encodingCtrl carrot.SubControl
	println("* start recording")
	{
		capturer.numImages = 0
		capturer.numProcessed = 0
		capturer.game.borderLight = ColorRed
		capturer.game.borderDark = ColorRedDark
		capturer.game.borderOnly = true
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
		ebiten.SetWindowDecorated(false)
		capturer.draw = capturer.drawActive
		capturer.running.Store(true)

		ctrl.Yield()

		fps := capturer.game.settings.FPS
		frameDuration := (1 / float64(fps)) * 1000 * 1000

		queue := CreateQueue[GifFrame](128)

		screenShotCtrl := ctrl.StartAsync(func(ctrl *carrot.Control) {
			lastShot := time.Now()
			for {
				bounds := GetWindowBounds()

				img, err := screenshot.CaptureRect(bounds)
				if err != nil {
					capturer.Error = err
					return
				}

				delay := int(time.Since(lastShot).Milliseconds() / 10)
				capturer.numImages++
				println("* screenshot", capturer.numImages)

				queue.Push(GifFrame{Image: img, CsDelay: delay})
				lastShot = time.Now()
				ctrl.Sleep(time.Duration(frameDuration) * time.Microsecond)
			}
		})
		encodingCtrl = ctrl.StartAsync(func(ctrl *carrot.Control) {
			for !queue.IsEmpty() || !screenShotCtrl.IsDone() {
				frame, ok := queue.Pop()
				if !ok {
					ctrl.Yield()
					continue
				}
				task := SaveOneGif(encoder, frame.Image, frame.CsDelay)
				ctrl.YieldUntil(task.IsDone)
				capturer.numProcessed++
				println("* saved", capturer.numProcessed)
			}
		})

		awaitEnter()
		screenShotCtrl.Cancel()

		ctrl.YieldUntil(screenShotCtrl.IsDone)

		if capturer.Error != nil {
			goto ERROR
		}
	}

	// saving
	println("* saving")
	{
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
		capturer.game.borderOnly = false
		capturer.draw = capturer.drawSaving

		now := time.Now()
		for {
			ctrl.Yield()
			if time.Since(now).Seconds() > 2 && encodingCtrl.IsDone() {
				break
			}
		}
	}

	// saved
	println("* saved")
	{
		capturer.draw = capturer.drawSaved
		now := time.Now()
		for {
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || time.Since(now).Seconds() > 2 {
				break
			}
			ctrl.Yield()
		}
	}

	goto START

}

func (capturer *GifCapturer) IsRunning() bool {
	return capturer.running.Load()
}

func (capturer *GifCapturer) Update() {
	capturer.script.Update()

	if !capturer.IsRunning() {
		s := &capturer.game.settings
		fps := s.FPS
		if inpututil.IsKeyJustPressed(ebiten.KeyMinus) && fps > 1 {
			fps--
		} else if inpututil.IsKeyJustPressed(ebiten.KeyEqual) && fps < 30 {
			fps++
		}
		if s.FPS != fps {
			s.FPS = fps
			capturer.game.scheduleSaveSettings()
		}
	}
}

func (capturer *GifCapturer) drawInactive(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorTeal
	capturer.scrp.Println("Ready")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to start")

	scrp.Font = capturer.game.smallFont
	scrp.Println("\n\n")
	scrp.Printf("%v FPS [-][+]", capturer.game.settings.FPS)
	scrp.Font = capturer.game.tinyFont

	scrp.Println("\n\n")
	scrp.Font = capturer.game.smallFont
	scrp.Println("Resize and position\nthis window to the area ")
	scrp.Println("where you want to capture.")
}

func (capturer *GifCapturer) drawActive(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorGreen
	capturer.scrp.Println("Recording")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to stop")
	scrp.Font = capturer.game.smallFont
	capturer.scrp.Printf("number of images: %v", capturer.numImages)

	scrp.Println("\n\n")
	scrp.Font = capturer.game.smallFont
	scrp.Println("You can now hide or minimize this window, or press F10 to show border only")
	scrp.Println("When you are done, return to this window.")
}

func (capturer *GifCapturer) drawSaving(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Printf("Saving to %v\n", capturer.saveFilename)
	scrp.Color = ColorWhite
	scrp.Font = capturer.game.smallFont
	capturer.scrp.Printf("Please wait: %v / %v", capturer.numProcessed, capturer.numImages)
}

func (capturer *GifCapturer) drawSaved(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Done!")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to continue")
}

func (capturer *GifCapturer) drawError(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Ruh-oh, Something broke")
	scrp.Color = ColorWhite
	capturer.scrp.Printf("%v", capturer.Error)
}

func (capturer *GifCapturer) Draw(screen *ebiten.Image) {
	scrp := capturer.scrp

	scrp.AlignX = 0b11

	scrp.Font = capturer.game.regularFont
	if capturer.draw != nil && !capturer.game.borderOnly {
		capturer.draw(screen)
	}

	if capturer.game.borderOnly && capturer.running.Load() {
		scrp.Font = capturer.game.tinyFont
		_, h := ebiten.WindowSize()

		s := fmt.Sprintf("%v", capturer.numImages)
		b := text.BoundString(scrp.Font, "9")
		w := b.Dx() * len(s)

		ebitenutil.DrawRect(
			screen,
			float64(0),
			float64(h-b.Dy()-scrp.Border),
			float64(w+scrp.Border),
			float64(b.Dy()+scrp.Border),
			ColorBlackTransparent,
		)
		scrp.PrintAt(0b0001, s)
	}

}

// TODO: use robogo to follow mouse
// TODO: try recording for an hour
