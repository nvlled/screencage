package main

import (
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/nvlled/screencage/lib"
)

func main() {
	log.SetOutput(os.Stderr)
	ebiten.SetTPS(30)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(false)
	ebiten.SetScreenTransparent(true)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("screen capture")

	game := lib.NewGame()
	game.Init()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
