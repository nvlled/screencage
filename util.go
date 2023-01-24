package main

import (
	"fmt"
	"image"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
)

func TrimExt(filename string) (baseFilename, ext string) {
	ext = filepath.Ext(filename)
	baseFilename = strings.TrimSuffix(filename, ext)
	return
}

func IncrementFilename(filename string) string {
	fileExt := filepath.Ext(filename)
	filename = strings.TrimSuffix(filename, fileExt)

	if filename == "" && fileExt != "" {
		filename, fileExt = fileExt, ""
	}

	i := len(filename) - 1
	if i < 0 {
		return ""
	}

	for ; i >= 0; i-- {
		ch := rune(filename[i])
		if !unicode.IsDigit(ch) {
			break
		}
	}

	currentNum := 0

	digits := filename[i+1:]
	filename = filename[0 : i+1]

	if filename[i] != '-' {
		filename += "-"
	}

	if n, err := strconv.Atoi(digits); err == nil {
		currentNum = n
	}

	currentNum++

	return fmt.Sprintf("%s%d%s", filename, currentNum, fileExt)
}

func GetWindowBounds() image.Rectangle {
	x, y := ebiten.WindowPosition()
	w, h := ebiten.WindowSize()
	return image.Rect(x, y, x+w, y+h)
}
