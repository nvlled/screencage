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
	"strings"

	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/nvlled/screencage/framerate"
	"github.com/sqweek/dialog"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var lightBorderImage *ebiten.Image
var darkBorderImage *ebiten.Image

var WindowTitle = "screen capture"

type Game struct {
	tickCounter int

	regularFont font.Face
	smallFont   font.Face
	tinyFont    font.Face

	scrp *ScreenPrint

	settingFilename string
	outputFilename  string
	settings        Settings

	err error

	mustSaveSettings bool

	capturer    Capturer
	gifCapturer *GifCapturer
	pngCapturer *PngCapturer

	borderOnly  bool
	borderLight color.Color
	borderDark  color.Color
}

func NewGame() *Game {
	game := &Game{
		borderLight: ColorTeal,
		borderDark:  ColorTealDark,
		scrp:        NewScreenPrint(),
	}
	game.gifCapturer = NewGifCapturer(game)
	game.pngCapturer = NewPngCapturer(game)
	return game
}

func (g *Game) Update() error {
	g.tickCounter++

	if g.mustSaveSettings && g.tickCounter%50 == 0 { // throttle by 50 frames
		g.onSettingsChanged()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		g.borderOnly = !g.borderOnly
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		filter := g.settings.OutputType.String()
		filename, err := dialog.File().
			Filter(filter, filter).
			Save()
		if err != nil && err != dialog.ErrCancelled {
			g.setError(err)
		} else if filename != "" {
			g.settings.OutputFilename = filename
			g.outputFilename = filename
			g.setOutputType(g.settings.OutputType)
			g.saveSettings()
		}
	}

	if g.capturer == nil || !g.capturer.IsRunning() {
		if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
			s := &g.settings
			outputType := (s.OutputType + 1) % OutputType_Size
			g.setOutputType(outputType)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
			ebiten.SetWindowDecorated(!ebiten.IsWindowDecorated())
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
			s := &g.settings
			s.OutputMethod = (s.OutputMethod + 1) % OutputMethod_Size
		}
	}

	if g.capturer != nil {
		g.capturer.Update()
	}

	return nil
}

func (g *Game) drawBorder(screen *ebiten.Image) {
	b := screen.Bounds()

	sw, sh := float64(b.Dx()-1), float64(b.Dy()-1)
	colors := []*ebiten.Image{lightBorderImage, darkBorderImage}

	for i, c := range colors {
		_ = i
		n := float64(i)
		op := ebiten.GeoM{}
		op.Scale(sw-n, 1)
		op.Translate(n, n)
		screen.DrawImage(c, &ebiten.DrawImageOptions{
			GeoM: op,
		})
		op.Reset()
		op.Scale(1, sh-n)
		op.Translate(n, n)
		screen.DrawImage(c, &ebiten.DrawImageOptions{
			GeoM: op,
		})
		op.Reset()
		op.Scale(sw-n, 1)
		op.Translate(n, sh-n)
		screen.DrawImage(c, &ebiten.DrawImageOptions{
			GeoM: op,
		})
		op.Reset()
		op.Scale(1, sh-n)
		op.Translate(sw-n, n)
		screen.DrawImage(c, &ebiten.DrawImageOptions{
			GeoM: op,
		})
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scrp.Reset(screen)

	if !g.borderOnly {
		b := screen.Bounds()
		ebitenutil.DrawRect(screen, 0, 0, float64(b.Dx()), float64(b.Dy()), color.RGBA{0, 0, 0, 150})
	}

	g.drawBorder(screen)

	if g.err != nil {
		g.scrp.PrintAt(0b1111, g.err.Error())
		return
	}

	var infoColor color.Color = ColorWhite
	if g.capturer != nil && g.capturer.IsRunning() {
		infoColor = ColorGray
	}

	if g.settings.WindowRect.H >= 250 && !g.borderOnly {
		g.scrp.AlignX = 0b10
		g.scrp.Font = g.tinyFont
		g.scrp.Color = infoColor
		g.scrp.PrintColumn(
			fmt.Sprintf("Output file [F5]: %v", g.outputFilename),
			"Toggle frame [F9]",
		)
		g.scrp.PrintColumn(
			fmt.Sprintf("Output method [F12]: %v", g.settings.OutputMethod),
			"Hide [F10]",
		)
		g.scrp.Printf("Output type [F8]: %v", g.settings.OutputType)
		g.scrp.Println("\n\n\n")
	}

	if g.capturer != nil {
		g.capturer.Draw(screen)
	}

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	wr := &g.settings.WindowRect
	x, y := ebiten.WindowPosition()

	if wr.W != outsideWidth || wr.H != outsideHeight || wr.X != x || wr.Y != y {
		g.mustSaveSettings = true
	}

	wr.X, wr.Y = x, y
	wr.W = outsideWidth
	wr.H = outsideHeight

	return outsideWidth, outsideHeight
}

func (g *Game) Init() {
	lightBorderImage = ebiten.NewImage(1, 1)
	lightBorderImage.Set(0, 0, g.borderLight)
	darkBorderImage = ebiten.NewImage(1, 1)
	darkBorderImage.Set(0, 0, g.borderDark)

	g.settingFilename = defaultSettingsFile

	g.loadSettings()
	g.setOutputType(g.settings.OutputType)

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
	outputType := defaultOutputType
	g.settings = Settings{
		OutputFilename: defaultOutputFileMp4,
		OutputType:     outputType,
		OutputMethod:   OutputMethodNewFile,
		WindowRect: Rect{
			X: 0,
			Y: 0,
			W: w,
			H: h,
		},
		FrameRate: defaultFrameRate,
	}

	g.outputFilename = g.settings.OutputFilename

	g.parseArgs()

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
		case OutputTypeGif:
			filename = defaultOutputFileGif
		case OutputTypePng:
			filename = defaultOutputFilePng
		}
		g.settings.OutputFilename = filename
	}

	g.outputFilename = g.settings.OutputFilename
}

func (g *Game) scheduleSaveSettings() {
	g.mustSaveSettings = true
}

func (g *Game) parseArgs() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		opt := args[i]
		if opt == "" {
			continue
		}
		if opt[0] != '-' {
			fmt.Printf("unknown option: %v\n", opt)
			os.Exit(1)
		}

		if i >= len(args)-1 {
			fmt.Printf("invalid option: %v needs a paramter\n", opt)
			os.Exit(1)
		}

		for len(opt) > 0 && opt[0] == '-' {
			opt = strings.TrimPrefix(opt, "-")
		}

		val := args[i+1]
		i++

		switch opt {
		case "config":
			g.settingFilename = val
		//case "filename":
		//  TODO: or maybe not
		default:
			fmt.Printf("unknown option: %v\n", opt)
		}
	}
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
	tinyFont, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size: 15,
		DPI:  dpi,
	})
	if err != nil {
		log.Fatal(err)
	}

	g.regularFont = regularFont
	g.smallFont = smallFont
	g.tinyFont = tinyFont
}

func (g *Game) setError(err error) {
	g.err = err
	println("error:", err.Error())
	debug.PrintStack()
}

func (g *Game) onSettingsChanged() {
	println("settings changed")
	wr := &g.settings.WindowRect
	g.saveSettings()
	g.mustSaveSettings = false
	ebiten.SetWindowTitle(fmt.Sprintf("%v %vx%v", WindowTitle, wr.W, wr.H))
}

func (g *Game) getNextOutFilename() (string, int) {
	outputFilename := g.settings.OutputFilename
	if g.settings.OutputMethod != OutputMethodNewFile {
		return outputFilename, 0
	}
	_, err := os.Stat(outputFilename)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			g.logError(err)
		}
		return outputFilename, 0
	}

	return NextLatestIncrementedFilename(outputFilename)
}

func (g *Game) logError(err error) {
	log.Println(err)
	debug.PrintStack()
}

func (g *Game) setOutputType(outputType OutputType) {
	s := &g.settings
	s.OutputType = outputType
	filename, _ := TrimExt(s.OutputFilename)
	s.OutputFilename = filename + "." + s.OutputType.String()
	g.outputFilename = s.OutputFilename

	switch outputType {
	case OutputTypeGif:
		g.capturer = g.gifCapturer
	case OutputTypePng:
		g.capturer = g.pngCapturer
	default:
		g.capturer = nil
	}
	g.adjustFrameRate()
}

func (g *Game) adjustFrameRate() {
	s := &g.settings
	unit := g.settings.FrameRate.Unit
	switch g.settings.OutputType {
	case OutputTypeGif:
		if unit != framerate.UnitSecond {
			s.FrameRate = defaultFrameRate
		}
		if s.FrameRate.Value > 30 {
			s.FrameRate.Value = 30
		}
	case OutputTypePng:
		if s.FrameRate.Value > 30 && s.FrameRate.Unit == framerate.UnitSecond {
			s.FrameRate.Value = 30
		} else if s.FrameRate.Value > 60 {
			s.FrameRate.Value = 60
		}
	}
}

func main() {
	ebiten.SetTPS(30)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(false)
	ebiten.SetScreenTransparent(true)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("screen capture")

	game := NewGame()
	game.Init()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
