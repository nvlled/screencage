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
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	defaultSettingsFile  = "screen-ebi-config.json"
	defaultOutputFileMp4 = "capture.mp4"
	defaultOutputFileGif = "capture.gif"
	defaultOutputFilePng = "capture.png"
)

// TODO: cli arg: settingsFilename
// TODO: keybindings
// TODO: screen capture

type Game struct {
	tickCounter int

	regularFont font.Face
	smallFont   font.Face

	scrp ScreenPrint

	settingFilename string
	settings        Settings

	err error

	WindowWidth  int
	WindowHeight int

	windowSizeChanged bool
}

type Rect struct {
	X int
	Y int
	W int
	H int
}

type OutputType int

const (
	OutputTypeMp4 OutputType = iota
	OutputTypeGif
	OutputTypePng
)

type OutputMethod int

const (
	OutputMethodNewFile = iota
	OutputMethodAbort
	OutputMethodOverwrite
)

type Settings struct {
	OutputFilename string       `json:"outputFilename"`
	OutputType     OutputType   `json:"outputType"`
	OutputMethod   OutputMethod `json:"outputMethod"`

	WindowRect Rect `json:"windowRect"`
}

func (otype OutputType) String() string {
	switch otype {
	case OutputTypeMp4:
		return "mp4"
	case OutputTypeGif:
		return "gif"
	case OutputTypePng:
		return "png"
	}
	return "invalid-output-type"
}

func (method OutputMethod) String() string {
	switch method {
	case OutputMethodAbort:
		return "abort"
	case OutputMethodNewFile:
		return "new file"
	case OutputMethodOverwrite:
		return "overwrite"
	}
	return "invalid-output-method"
}

func (g *Game) Update() error {
	g.tickCounter++

	if g.windowSizeChanged && g.tickCounter%50 == 0 { // throttle by 50 frames
		g.onWindowSizeChange()
	}

	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		os.Exit(0)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scrp.Reset(screen)

	b := screen.Bounds()
	ebitenutil.DebugPrint(screen, "Hello, World!")

	sw, sh := float64(b.Dx()-1), float64(b.Dy()-1)
	color1 := color.RGBA{0, 255, 255, 255}
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
	g.scrp.Printf("Output file [F5]: %v", g.settings.OutputFilename)
	g.scrp.Printf("Output type [F8]: %v", g.settings.OutputType)
	g.scrp.Printf("Output method [F12]: %v", g.settings.OutputMethod)

	g.scrp.Color = color1
	g.scrp.Font = g.regularFont
	g.scrp.AlignX = 0b11
	g.scrp.Printf("\n\n\nStatus: %v\n", "ready")

	g.scrp.AlignX = 0b11
	g.scrp.Color = color.White
	g.scrp.Font = g.regularFont
	g.scrp.Printf("Press [enter] to start")

	g.scrp.Println("\n\n\n\n\n")
	g.scrp.Println("Resize and position this window to the area ")
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
	w, h := ebiten.ScreenSizeInFullscreen()
	g.settingFilename = defaultSettingsFile
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
	g.saveSettings()

	g.windowSizeChanged = false
}

func main() {

	ebiten.SetFPSMode(ebiten.FPSModeVsyncOffMinimum)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(false)
	ebiten.SetScreenTransparent(true)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")

	game := &Game{}
	game.Init()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
