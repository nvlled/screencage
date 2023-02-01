package screencage

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync/atomic"
	"time"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/kbinani/screenshot"
	"github.com/nvlled/carrot"
	gif "github.com/nvlled/gogif"
	"github.com/nvlled/screencage/framerate"
)

type GifCapturer struct {
	saveFilename string

	numImages    int
	numProcessed int

	running atomic.Bool

	draw func(*ebiten.Image)

	script *carrot.Script

	game *App
	scrp *ScreenPrint

	lastDraw int64

	Err error
}

func NewGifCapturer(game *App) *GifCapturer {
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

func (capturer *GifCapturer) coroutine(ctrl *carrot.Control) {
START:
	for {
		println("* inactive")
		capturer.game.borderLight = ColorTeal
		capturer.game.borderDark = ColorTealDark
		capturer.running.Store(false)
		capturer.draw = capturer.drawInactive
		capturer.numImages = 0
		capturer.numProcessed = 0

		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			ctrl.Yield()
		}

		for {
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				capturer.Err = capturer.startRecording(ctrl)
				break
			}
			if capturer.Err != nil {
				goto ERROR
			}

			ctrl.Yield()
		}
	}

ERROR:
	println("error", capturer.Err.Error())
	capturer.draw = capturer.drawError
	awaitEnter(ctrl)
	capturer.Err = nil

	goto START

}

func (capturer *GifCapturer) startRecording(ctrl *carrot.Control) error {
	capturer.saveFilename, _ =
		capturer.game.getNextOutFilename()

	file, err := os.OpenFile(capturer.saveFilename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gif.NewStreamEncoder(file, &gif.StreamEncoderOptions{})

	// recording
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

		awaitNextDraw(ctrl, &capturer.lastDraw)

		queue := CreateQueue[GifFrame](128)

		var err error

		screenShotCtrl := ctrl.StartAsync(func(ctrl *carrot.Control) {
			err = capturer.startScreenShotLoop(&queue, ctrl)
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

				if task.Err != nil {
					err = task.Err
					return
				}

				capturer.numProcessed++
			}
		})

		awaitEnter(ctrl)
		for !inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if err != nil {
				return err
			}
			ctrl.Yield()
		}

		screenShotCtrl.Cancel()

		for !screenShotCtrl.IsDone() {
			if err != nil {
				return err
			}
			ctrl.Yield()
		}
		//ctrl.YieldUntil(screenShotCtrl.IsDone)
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
		if err := encoder.Close(); err != nil {
			return err
		}
	}

	// saved
	println("* saved")
	{
		capturer.draw = capturer.drawSaved
		now := time.Now()
		for {
			ctrl.Yield()
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || time.Since(now).Seconds() > 2 {
				break
			}
		}
	}

	return nil
}

func (capturer *GifCapturer) startScreenShotLoop(queue *Queue[GifFrame], ctrl *carrot.Control) error {
	lastShot := time.Now()
	rate := capturer.game.settings.FrameRate
	frameDuration := rate.Duration()
	for {
		bounds := GetWindowBounds()

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return err
		}

		delay := int(time.Since(lastShot).Milliseconds() / 10)
		if delay > 500 {
			delay = 500
		}
		capturer.numImages++
		println("* screenshot", capturer.numImages, delay)

		queue.Push(GifFrame{Image: img, CsDelay: delay})
		lastShot = time.Now()
		ctrl.Sleep(frameDuration)
	}
}

func (capturer *GifCapturer) IsRunning() bool {
	return capturer.running.Load()
}

func (capturer *GifCapturer) Update() {
	capturer.script.Update()

	if !capturer.IsRunning() {
		s := &capturer.game.settings

		stepSize := 1
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			stepSize = 10
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			s.FrameRate.Value -= stepSize
			capturer.game.scheduleSaveSettings()
		} else if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			s.FrameRate.Value += stepSize
			capturer.game.scheduleSaveSettings()
		}
		s.FrameRate.Clamp(1, 30)

		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			s.FrameRate.Unit = (s.FrameRate.Unit + 1) % framerate.Unit_End
		}
	}
}

func (capturer *GifCapturer) drawInactive(screen *ebiten.Image) {
	scrp := capturer.scrp
	s := capturer.game.settings
	scrp.Color = ColorTeal
	capturer.scrp.Println("Ready")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to start recording")

	scrp.Font = capturer.game.smallFont
	scrp.Println("\n\n")
	scrp.Printf("rate: %v", s.FrameRate.String())
	scrp.Printf("controls: [up][down] or [backspace]")
	scrp.Printf("shift can be used [up][down]")
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
	capturer.scrp.Printf("%v", capturer.Err)
}

func (capturer *GifCapturer) Draw(screen *ebiten.Image) {
	capturer.lastDraw = time.Now().UnixMilli()
	scrp := capturer.scrp

	scrp.AlignX = 0b11

	scrp.Font = capturer.game.regularFont
	if capturer.draw != nil && !capturer.game.borderOnly {
		capturer.draw(screen)
	}

	/*
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
	*/
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
