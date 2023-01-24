package main

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"sync/atomic"
	"time"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	fps int
	//rect image.Rectangle

	//state atomic.Int32

	//queue chan int
	//halt  chan struct{}

	//images []*image.RGBA
	//delays []int

	//lastShot   time.Time
	//lastUpdate time.Time

	draw func(*ebiten.Image)

	script *carrot.Script

	game *Game
	scrp *ScreenPrint

	Error error
}

func NewGifCapturer(game *Game) *GifCapturer {
	capturer := &GifCapturer{
		game: game,
		scrp: &game.scrp,
	}
	capturer.script = carrot.Start(capturer.coroutine)
	return capturer
}

/*
func (capturer *GifCapturer) SetBounds(x, y, w, h int) {
	r := &capturer.rect
	r.Min.X = x
	r.Min.Y = y
	r.Max.X = x + w
	r.Max.Y = y + h
}
*/

type ScreenShotTask struct {
	Image *image.RGBA
	Error error
	done  atomic.Bool

	CsElapsed int // Cs = Centiseconds
}

func (task *ScreenShotTask) IsDone() bool {
	return task.done.Load()
}

func StartScreenshot(bounds image.Rectangle) Task[GifFrame] {
	task := Task[GifFrame]{}
	go func() {
		startTime := time.Now()
		img, err := screenshot.CaptureRect(bounds)
		duration := time.Since(startTime)

		if err != nil {
			task.Error = err
		} else {
			task.Result.Image = img
			task.Result.CsDelay = int(duration.Abs().Milliseconds()) / 10
			task.done.Store(true)

		}
	}()

	return task
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

func SaveGif(filename string, frames []GifFrame) Task[carrot.Void] {
	task := Task[carrot.Void]{}
	go func() {
		p := make([]color.Color, 0, 256)
		gifImage := []*image.Paletted{}
		q := quantize.MedianCutQuantizer{}

		delays := make([]int, len(frames))
		for _, frame := range frames {
			img := frame.Image
			delays = append(delays, frame.CsDelay)
			palleted := &image.Paletted{
				Pix:     img.Pix,
				Stride:  img.Stride,
				Rect:    img.Rect,
				Palette: q.Quantize(p, img),
			}
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

			task.Error = gif.EncodeAll(file, g)
		}

	}()
	return task
}

func (capturer *GifCapturer) coroutine(in *carrot.Invoker) {
	awaitEnter := func() {
		in.UntilFunc(func() bool { return ebiten.IsKeyPressed(ebiten.KeyEnter) })
	}

ERROR:
	println(capturer.Error)
	capturer.draw = capturer.drawError
	awaitEnter()
	capturer.Error = nil

START:

	// inactive
	capturer.draw = capturer.drawInactive
	awaitEnter()

	// active

	bounds := GetWindowBounds()
	var frames []GifFrame
	screenshotCo := carrot.Start(func(in *carrot.Invoker) {
		var task Task[GifFrame]
		task.Finish()
		for {
			in.UntilFunc(task.IsDone)
			if task.Error != nil {
				capturer.Error = task.Error
				return
			}

			delay := task.Result.CsDelay
			if len(frames) == 0 {
				delay = 0
			}
			frames = append(frames, GifFrame{
				Image:   task.Result.Image,
				CsDelay: delay,
			})

			task = StartScreenshot(bounds)

			frameDuration := (1 / float64(capturer.fps)) * 1000 * 1000
			in.Sleep(time.Duration(frameDuration) * time.Microsecond)
		}
	})

	awaitEnter()
	screenshotCo.Cancel()
	in.UntilFunc(screenshotCo.IsDone)
	if capturer.Error != nil {
		goto ERROR
	}

	// saving
	capturer.draw = capturer.drawSaving
	filename := capturer.game.settings.OutputFilename
	saveTask := SaveGif(filename, frames)
	in.UntilFunc(saveTask.IsDone)

	// saved
	capturer.draw = capturer.drawSaved

	now := time.Now()
	for {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || time.Since(now).Seconds() > 5 {
			break
		}
	}

	goto START

}

/*
func (capturer *GifCapturer) screenshot() {
	if len(capturer.images) == 0 {
		capturer.lastShot = time.Now()
	}

	img, err := screenshot.CaptureRect(capturer.rect)
	duration := time.Since(capturer.lastShot)

	if err != nil {
		if capturer.Error == nil {
			capturer.Error = err
		} else {
			millis := int(duration.Abs().Milliseconds())
			capturer.images = append(capturer.images, img)
			capturer.delays = append(capturer.delays, millis/10)
		}
	}
}
*/

/*
func (capturer *GifCapturer) Screenshot() {
	if capturer.state.Load() != GifCapturerStateRunning {
		return
	}
	capturer.queue <- GifCapturerActionScreenshot
}

func (capturer *GifCapturer) Dispose(filename string) {
	if capturer.state.Load() == GifCapturerStateDisposed {
		return
	}
	capturer.state.Store(GifCapturerStateDisposed)
	capturer.halt <- struct{}{}
	capturer.images = nil
	capturer.delays = nil
}
*/

/*
func (capturer *GifCapturer) save() {
	images := capturer.images
	delays := capturer.delays

	capturer.images = nil
	capturer.delays = nil

	p := make([]color.Color, 0, 256)
	gifImage := []*image.Paletted{}
	q := quantize.MedianCutQuantizer{}
	for _, img := range images {
		palleted := &image.Paletted{
			Pix:     img.Pix,
			Stride:  img.Stride,
			Rect:    img.Rect,
			Palette: q.Quantize(p, img),
		}
		gifImage = append(gifImage, palleted)
	}

	g := &gif.GIF{
		Image: gifImage,
		Delay: delays,
	}
	file, err := os.OpenFile(capturer.filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		capturer.Error = err
	} else {
		capturer.Error = gif.EncodeAll(file, g)
	}

}
func (capturer *GifCapturer) Save(filename string) {
	if !capturer.state.CompareAndSwap(GifCapturerStateRunning, GifCapturerStateSaving) {
		return
	}

	capturer.filename = filename
	capturer.queue <- GifCapturerActionSave
}
*/

/*

// Using coroutine definitely is more clean,
// and the functions are reusable and modular.
// No need to worry about channel deadlocks too.
func coroutine(in) {
	awaitEnter := func() {
		in.UntilFunc(ebiten.IsKeyPressed)
	}

inactive:
	capturer.draw = capturer.drawInactive
	awaitEnter()


active:
	var images []*image.RGBA
	var delays []int = {}int{}
	screenshotCo := in.Start(func(in) {
		task = ...
		for {

			if task != nil {
				in.UntilFunc(task.IsDone)
				delay := task.CsecDuration
				if len(images) == 0{
					delay = 0
				}
				delays = append(delays, task.CsecDuration)
				images = append(images, task.Image)
			}

			task = screenshot()

			frameDuration := (1 / float64(capturer.fps))
			in.Sleep(frameDuration)
		}
	})

	awaitEnter()
	screenshotCo.Cancel()
	in.UntilFunc(screenshotCo.IsDone)

saving:
	capturer.draw = capturer.drawSaving
	saveTask = SaveGif(images, delays)
	in.UntilFunc(saveTask.IsDone)

saved:
	capturer.draw = capturer.drawDone

	now := time.Now()
	for capturer.IsSaving() {
		if input.IsKeyJustPressed(Enter) || time.Since(now).Seconds() > 5 {
			break
		}
	}

	goto start:

disposed:
}

func (capturer *GifCapturer) updateSaving() {
	if capturer.fps < 60 {
		frameDuration := (1 / float64(capturer.fps)) * 1000 * 1000
		if time.Since(capturer.lastUpdate).Microseconds() < int64(frameDuration) {
			return
		}
	}

	capturer.lastUpdate = time.Now()
	capturer.Screenshot()
}

*/

func (capturer *GifCapturer) Update() {
	/*
		switch capturer.state.Load() {
		case GifCapturerStateStopped:
			capturer.updateStopped()
		case GifCapturerStateRunning:
			capturer.updateRunning()
		case GifCapturerStateSaving:
			capturer.updateSaving()
		}
	*/
}

func (capturer *GifCapturer) drawInactive(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorTeal
	capturer.scrp.Println("Status: Ready")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to start")
}
func (capturer *GifCapturer) drawActive(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorGreen
	capturer.scrp.Println("Status: Recording")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to stop")
}

func (capturer *GifCapturer) drawSaving(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Status: Saving")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press wait...")
}

func (capturer *GifCapturer) drawSaved(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Status: done")
	scrp.Color = ColorWhite
	capturer.scrp.Println("Press [enter] to continue")
}

func (capturer *GifCapturer) drawError(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.Color = ColorWhite
	capturer.scrp.Println("Status: something broke")
	scrp.Color = ColorWhite
	capturer.scrp.Printf("%v", capturer.Error)
}

func (capturer *GifCapturer) Draw(screen *ebiten.Image) {
	scrp := capturer.scrp
	scrp.AlignX = 0b10
	if capturer.draw != nil {
		capturer.draw(screen)
	}
	/*
		g.scrp.Font = g.smallFont

		g.scrp.Color = ColorTeal
		g.scrp.Font = g.regularFont
		g.scrp.AlignX = 0b11
		g.scrp.Printf("\n\n\nStatus: %v\n", "ready")

		g.scrp.AlignX = 0b11
		g.scrp.Color = color.White
		g.scrp.Font = g.regularFont
		g.scrp.Printf("Press [enter] to start")

		g.scrp.Println("\n\n\n\n\n")
		g.scrp.Font = g.smallFont
		g.scrp.Println("Resize and position\nthis window to the area ")
		g.scrp.Println("where you want to capture.")
	*/
}
