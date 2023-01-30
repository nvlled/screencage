package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"sync/atomic"
	"time"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/kbinani/screenshot"
	"github.com/nvlled/carrot"
	"github.com/xyproto/palgen"
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

type Task[T any] struct {
	Result T
	Error  error
	done   atomic.Bool
}

func (task *Task[T]) Finish()      { task.done.Store(true) }
func (task *Task[T]) IsDone() bool { return task.done.Load() }

type Void struct{}

func SaveGif(filename string, frames []GifFrame, onProcessOpt ...func()) *Task[Void] {
	task := &Task[Void]{}
	go func() {
		defer task.Finish()

		gifImage := []*image.Paletted{}
		quantizer := quantize.MedianCutQuantizer{}

		_ = quantizer
		_ = palgen.Generate

		onProcess := func() {}
		if len(onProcessOpt) > 0 {
			onProcess = onProcessOpt[0]
		}

		// TODO: run parallel
		delays := make([]int, len(frames))
		for i, frame := range frames {
			img := frame.Image

			delays[i] = frame.CsDelay

			emptyPalette := make([]color.Color, 0, 256)
			pal := quantizer.Quantize(emptyPalette, img)

			palleted := image.NewPaletted(img.Rect, pal)
			draw.Src.Draw(palleted, img.Bounds(), img, image.Point{})

			println("processed image", i, "delay=", frame.CsDelay)
			onProcess()
			gifImage = append(gifImage, palleted)
		}

		g := &gif.GIF{
			Image: gifImage,
			Delay: delays,
		}

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			task.Error = err
		} else {
			println("writing gif to file", filename)
			task.Error = gif.EncodeAll(file, g)
			file.Close()
		}

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

START:
	// inactive
	println("* inactive")
	capturer.game.borderLight = ColorTeal
	capturer.game.borderDark = ColorTealDark
	capturer.running.Store(false)
	capturer.draw = capturer.drawInactive
	awaitEnter()

	// active
	println("* start recording")
	var frames []GifFrame
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

		fps := capturer.game.settings.FPS
		frameDuration := (1 / float64(fps)) * 1000 * 1000

		screenshotCo := ctrl.StartAsync(func(ctrl *carrot.Control) {
			lastShot := time.Now()
			for {
				bounds := GetWindowBounds()

				img, err := screenshot.CaptureRect(bounds)
				if err != nil {
					capturer.Error = err
					return
				}

				delay := int(time.Since(lastShot).Milliseconds() / 10)
				if len(frames) == 0 {
					delay = 0
				}

				capturer.numImages++
				frames = append(frames, GifFrame{
					Image:   img,
					CsDelay: delay,
				})
				lastShot = time.Now()

				println("screenshot taken")

				ctrl.Sleep(time.Duration(frameDuration) * time.Microsecond)
			}
		})

		awaitEnter()
		screenshotCo.Cancel()
		ctrl.YieldUntil(screenshotCo.IsDone)

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
		capturer.saveFilename = capturer.game.getNextOutFilename()
		saveTask := SaveGif(capturer.saveFilename, frames, func() { capturer.numProcessed++ })
		ctrl.YieldUntil(saveTask.IsDone)
		if saveTask.Error != nil {
			capturer.Error = saveTask.Error
			goto ERROR
		}

		now := time.Now()
		for {
			ctrl.Yield()
			if time.Since(now).Seconds() > 2 && saveTask.IsDone() {
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

ERROR:
	println(capturer.Error)
	capturer.draw = capturer.drawError
	awaitEnter()
	capturer.Error = nil

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

// TODO: it runs out of memory when I leave it recording for too long
// So the problem is that the builtin gif package doesn't allow
// streaming to a file, so I just accumulate an increasing array
// of images. I could either fork gif to allow streaming,
// or find another library that does this.
// Forking or changing it myself seems like a better option
// since it's just a small change.
// Well, I can't just fork it since it part of the stlib.
