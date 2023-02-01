package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"sync/atomic"
	"time"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/kbinani/screenshot"
	"github.com/nvlled/carrot"
	"github.com/nvlled/screencage/framerate"
)

type PngCapturer struct {
	saveFilename string

	numImages    int
	numProcessed int

	imageCounter int

	running atomic.Bool

	draw func(*ebiten.Image)

	script *carrot.Script

	game *Game
	scrp *ScreenPrint

	lastDraw int64

	Err error
}

func NewPngCapturer(game *Game) *PngCapturer {
	capturer := &PngCapturer{
		game: game,
		scrp: game.scrp,
	}
	capturer.script = carrot.Start(capturer.coroutine)
	return capturer
}

type PngFrame struct {
	Image   *image.RGBA
	CsDelay int
}

func ScreenshotAndSave(filename string) *Task[Void] {
	task := &Task[Void]{}
	go func() {
		defer task.Finish()
		bounds := GetWindowBounds()
		image, err := screenshot.CaptureRect(bounds)
		if err != nil {
			task.Err = err
			return
		}
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			task.Err = err
			return
		}

		defer file.Close()
		task.Err = png.Encode(file, image)
	}()

	return task
}

func SaveOnePng(filename string, img *image.RGBA) *Task[Void] {
	task := &Task[Void]{}
	go func() {
		defer task.Finish()

		quantizer := quantize.MedianCutQuantizer{}
		emptyPalette := make([]color.Color, 0, 256)
		pal := quantizer.Quantize(emptyPalette, img)

		palleted := image.NewPaletted(img.Rect, pal)
		draw.Src.Draw(palleted, img.Bounds(), img, image.Point{})

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			task.Err = err
		} else {
			task.Err = png.Encode(file, img)
		}
	}()

	return task
}

func (capturer *PngCapturer) coroutine(ctrl *carrot.Control) {
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
			if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
				capturer.Err = capturer.startSingleScreenShot(ctrl)
				break
			} else if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				capturer.Err = capturer.startMultiScreenShot(ctrl)
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

func (capturer *PngCapturer) startSingleScreenShot(ctrl *carrot.Control) error {
	println("starting screenshot")
	capturer.game.borderOnly = true
	awaitNextDraw(ctrl, &capturer.lastDraw)

	filename, _ := capturer.game.getNextOutFilename()
	task := ScreenshotAndSave(filename)
	println("screenshot done")
	ctrl.YieldUntil(task.IsDone)
	capturer.game.borderOnly = false
	if task.Err != nil {
		return task.Err
	}

	return nil
}

func (capturer *PngCapturer) startMultiScreenShot(ctrl *carrot.Control) error {
	capturer.saveFilename, capturer.imageCounter =
		capturer.game.getNextOutFilename()

	file, err := os.OpenFile(capturer.saveFilename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	var savingCtrl carrot.SubControl

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

		queue := CreateQueue[*image.RGBA](128)

		var err error

		screenShotCtrl := ctrl.StartAsync(func(ctrl *carrot.Control) {
			err = capturer.startCaptureLoop(&queue, ctrl)
		})

		savingCtrl = ctrl.StartAsync(func(ctrl *carrot.Control) {
			for !queue.IsEmpty() || !screenShotCtrl.IsDone() {
				img, ok := queue.Pop()
				if !ok {
					ctrl.Yield()
					continue
				}

				filename := ReplaceIncrementedFilename(capturer.saveFilename, capturer.imageCounter)
				capturer.imageCounter++

				task := SaveOnePng(filename, img)
				ctrl.YieldUntil(task.IsDone)

				if task.Err != nil {
					err = task.Err
					return
				}

				capturer.numProcessed++
				println("* saved", capturer.numProcessed)
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
	}

	println("* saving")
	{
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
		capturer.game.borderOnly = false
		capturer.draw = capturer.drawSaving

		now := time.Now()
		for {
			ctrl.Yield()
			if time.Since(now).Seconds() > 2 && savingCtrl.IsDone() {
				break
			}
		}
	}

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

func (capturer *PngCapturer) startCaptureLoop(queue *Queue[*image.RGBA], ctrl *carrot.Control) error {
	rate := capturer.game.settings.FrameRate
	frameDuration := rate.Duration()
	for {
		bounds := GetWindowBounds()

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return err
		}

		capturer.numImages++
		println("* screenshot", capturer.numImages)

		queue.Push(img)
		ctrl.Sleep(frameDuration)
	}
}

func (capturer *PngCapturer) IsRunning() bool {
	return capturer.running.Load()
}

func (capturer *PngCapturer) Update() {
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

func (capturer *PngCapturer) drawInactive(screen *ebiten.Image) {
	scrp := capturer.scrp
	s := capturer.game.settings
	scrp.Color = ColorTeal
	capturer.scrp.Println("Ready")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [space] to take one screenshot")
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

func (capturer *PngCapturer) drawActive(screen *ebiten.Image) {
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

func (capturer *PngCapturer) drawSaving(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Printf("Saving to %v\n", capturer.saveFilename)
	scrp.Color = ColorWhite
	scrp.Font = capturer.game.smallFont
	capturer.scrp.Printf("Please wait: %v / %v", capturer.numProcessed, capturer.numImages)
}

func (capturer *PngCapturer) drawSaved(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Done!")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to continue")
}

func (capturer *PngCapturer) drawError(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Ruh-oh, Something broke")
	scrp.Color = ColorWhite
	capturer.scrp.Printf("%v", capturer.Err)
}

func (capturer *PngCapturer) Draw(screen *ebiten.Image) {
	capturer.lastDraw = time.Now().UnixMilli()
	scrp := capturer.scrp

	scrp.AlignX = 0b11

	scrp.Font = capturer.game.regularFont
	if capturer.draw != nil && !capturer.game.borderOnly {
		capturer.draw(screen)
	}
}
