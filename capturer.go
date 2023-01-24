package main

import "github.com/hajimehoshi/ebiten/v2"

//type CapturerID int
//ID() CapturerID

const (
	CapturerIDGif = iota + 1
	CapturerIDPng
)

type Capturer interface {
	Update()
	Draw(*ebiten.Image)
}
