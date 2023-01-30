package main

import "github.com/hajimehoshi/ebiten/v2"

const (
	CapturerIDGif = iota + 1
	CapturerIDPng
)

type Capturer interface {
	Update()
	Draw(*ebiten.Image)
	IsRunning() bool
}
