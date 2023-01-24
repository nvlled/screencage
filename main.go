package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"log"
	"os"
	"runtime/debug"

	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/sqweek/dialog"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type Game struct {
	tickCounter int

	regularFont font.Face
	smallFont   font.Face

	scrp ScreenPrint

	settingFilename string
	outputFilename  string
	settings        Settings

	err error

	WindowWidth  int
	WindowHeight int

	windowSizeChanged bool
	capturing         bool

	capturer Capturer
}

func (g *Game) Update() error {
	g.tickCounter++

	g.updateCapture()

	if g.windowSizeChanged && g.tickCounter%50 == 0 { // throttle by 50 frames
		g.onWindowSizeChange()
	}

	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		os.Exit(0)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		filter := g.settings.OutputType.String()
		filename, err := dialog.File().
			Filter(filter, filter).
			SetStartFile(g.outputFilename).
			Save()
		if err != nil && err != dialog.ErrCancelled {
			g.setError(err)
		} else if filename != "" {
			g.settings.OutputFilename = filename
			g.saveSettings()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
		s := &g.settings
		s.OutputType = (s.OutputType + 1) % OutputType_Size
		s.OutputFilename, _ = TrimExt(s.OutputFilename)
		s.OutputFilename = s.OutputFilename + "." + s.OutputType.String()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		ebiten.SetWindowDecorated(!ebiten.IsWindowDecorated())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		s := &g.settings
		s.OutputMethod = (s.OutputMethod + 1) % OutputMethod_Size
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.startCapture()
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scrp.Reset(screen)

	b := screen.Bounds()
	ebitenutil.DebugPrint(screen, "Hello, World!")

	sw, sh := float64(b.Dx()-1), float64(b.Dy()-1)
	color1 := ColorTeal
	//color2 := color.RGBA{0, 40, 0, 255}

	ebitenutil.DrawRect(screen, 0, 0, sw, sh, color.RGBA{0, 0, 0, 150})

	ebitenutil.DrawLine(screen, 0, 0, sw, 0, color1)
	ebitenutil.DrawLine(screen, 1, 0, 1, sh, color1)
	ebitenutil.DrawLine(screen, sw, 0, sw, sh, color1)
	ebitenutil.DrawLine(screen, 0, sh, sw, sh, color1)

	//ebitenutil.DrawLine(screen, 1, 1, sw-1, 1, color2)
	//ebitenutil.DrawLine(screen, 2, 2, 2, sh-2, color2)
	//ebitenutil.DrawLine(screen, 2, sh-2, sw-2, sh-2, color2)
	//ebitenutil.DrawLine(screen, sw-2, 1, sw-2, sh-1, color2)

	if g.err != nil {
		g.scrp.PrintAt(0b1111, g.err.Error())
		return
	}

	//msg := "some text here"
	//textB := text.BoundString(g.font, msg)
	//text.Draw(screen, msg, g.font, int(sw/2)-textB.Dx()/2-1, int(sh/2)-textB.Dy()/2-1, color1)

	g.scrp.AlignX = 0b10
	g.scrp.Font = g.smallFont
	g.scrp.Printf("Output file [F5]: %v", g.outputFilename)
	g.scrp.Printf("Output type [F8]: %v", g.settings.OutputType)
	g.scrp.Printf("Output method [F12]: %v", g.settings.OutputMethod)

	if g.capturer != nil {
		g.capturer.Draw(screen)
	}

	g.scrp.Println("\n\n\n\n\n")
	g.scrp.Font = g.smallFont
	g.scrp.Println("Resize and position\nthis window to the area ")
	g.scrp.Println("where you want to capture.")

	g.scrp.Font = g.smallFont
	g.scrp.PrintAt(0b0101, "some text here")

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	if g.WindowWidth != outsideWidth || g.WindowHeight != outsideHeight {
		g.windowSizeChanged = true
	}

	g.WindowWidth = outsideWidth
	g.WindowHeight = outsideHeight

	return outsideWidth, outsideHeight
}

func (g *Game) Init() {
	g.settingFilename = defaultSettingsFile

	g.loadSettings()

	wr := g.settings.WindowRect
	ebiten.SetWindowPosition(wr.X, wr.Y)
	ebiten.SetWindowSize(wr.W, wr.H)
	fmt.Printf("%+v\n", wr)

	g.loadFonts()
	g.scrp.Font = g.regularFont

	g.scrp.Border = 20
	g.scrp.Color = color.White
	g.scrp.LineSpacing = 10
}

func (g *Game) loadSettings() {
	w, h := ebiten.ScreenSizeInFullscreen()
	g.settings = Settings{
		OutputFilename: defaultOutputFileMp4,
		OutputType:     OutputTypeMp4,
		OutputMethod:   OutputMethodNewFile,
		WindowRect: Rect{
			X: 0,
			Y: 0,
			W: w,
			H: h,
		},
	}
	g.outputFilename = g.getNextOutFilename()

	if os.Getenv("screenebi_config") != "" {
		g.settingFilename = os.Getenv("screenebi_config")
	}

	file, err := os.Open(g.settingFilename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}

		g.setError(err)
		return
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&g.settings)
	if err != nil {
		g.setError(err)
		return
	}

	if g.settings.OutputFilename == "" {
		filename := ""
		switch g.settings.OutputType {
		case OutputTypeMp4:
			filename = defaultOutputFileMp4
		case OutputTypeGif:
			filename = defaultOutputFileGif
		case OutputTypePng:
			filename = defaultOutputFilePng
		}
		g.settings.OutputFilename = filename
	}

	g.outputFilename = g.getNextOutFilename()
}

func (g *Game) saveSettings() {
	file, err := os.OpenFile(g.settingFilename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		g.setError(err)
		return
	}

	encoder := json.NewEncoder(file)
	err = encoder.Encode(g.settings)
	if err != nil {
		g.setError(err)
		return
	}
}

func (g *Game) loadFonts() {
	tt, err := opentype.Parse(fonts.MPlus1pRegular_ttf)
	if err != nil {
		log.Fatal(err)
	}

	const dpi = 72
	regularFont, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	smallFont, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    18,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	g.regularFont = regularFont
	g.smallFont = smallFont
}

func (g *Game) setError(err error) {
	g.err = err
	println("error:", err.Error())
	debug.PrintStack()
}

func (g *Game) onWindowSizeChange() {
	println("window size changed")

	wr := &g.settings.WindowRect
	wr.X, wr.Y = ebiten.WindowPosition()
	wr.W, wr.H = ebiten.WindowSize()

	g.saveSettings()

	g.windowSizeChanged = false
}

func (g *Game) getNextOutFilename() string {
	outputFilename := g.settings.OutputFilename
	if g.settings.OutputMethod != OutputMethodNewFile {
		return outputFilename
	}
	_, err := os.Stat(outputFilename)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			g.logError(err)
		}
		return outputFilename
	}

	return IncrementFilename(outputFilename)
}

func (g *Game) logError(err error) {
	log.Println(err)
	debug.PrintStack()
}

func (g *Game) updateCaptureGif() {
	if g.captureGif != nil {
		g.captureGif.Screenshot()
	}
}

func (g *Game) updateCapture() {
	if !g.capturing {
		return
	}

	if g.settings.OutputType == OutputTypeGif {
		g.updateCaptureGif()
	}
}

func (g *Game) startCapture() {
	if g.settings.OutputType == OutputTypeGif {
		capturer := NewGifCapturer()
		x, y := ebiten.WindowPosition()
		w, h := ebiten.WindowSize()
		capturer.SetBounds(x, y, w, h)
		g.capturer = capturer
	}

	g.capturing = true
}

func main() {

	//ebiten.SetRunnableOnUnfocused(false)
	ebiten.SetFPSMode(ebiten.FPSModeVsyncOffMinimum)

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(false)
	ebiten.SetScreenTransparent(true)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("screen capture")

	game := &Game{}
	game.Init()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

/*

TODO:

interface Capturer {
	Start()
	Update()
	StopAndSave()
}

type GifCapturer struct {}
type PngCapturer struct {}

*/

// TODO: keybindings, save settings on press
// TODO: option to show window frame,
//       since some OS doesn't have keyshortcuts for moving/resizing windows

// https://github.com/kbinani/screenshot
// https://pkg.go.dev/image/gif#EncodeAll
// use to get pallete from rgba
// https://github.com/ericpauley/go-quantize/
