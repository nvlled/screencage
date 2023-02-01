package main

import (
	"fmt"
	"time"

	"github.com/nvlled/screen-ebi/framerate"
)

type Rect struct {
	X int
	Y int
	W int
	H int
}

const (
	defaultSettingsFile  = "screen-ebi-config.json"
	defaultOutputFileMp4 = "capture.mp4"
	defaultOutputFileGif = "capture.gif"
	defaultOutputFilePng = "capture.png"
)

var defaultFrameRate = framerate.T{5, framerate.UnitSecond}

const defaultOutputType = OutputTypeGif

type OutputType int

const (
	OutputTypeGif OutputType = iota
	OutputTypePng

	// TODO:
	//OutputTypeMp4

	OutputType_Size
)

func (otype OutputType) String() string {
	switch otype {
	case OutputTypeGif:
		return "gif"
	case OutputTypePng:
		return "png"
	}
	return "invalid-output-type"
}

type OutputMethod int

const (
	OutputMethodOverwrite = iota
	OutputMethodNewFile

	OutputMethod_Size
)

func (method OutputMethod) String() string {
	switch method {
	case OutputMethodNewFile:
		return "new file"
	case OutputMethodOverwrite:
		return "overwrite existing file"
	}
	return "invalid-output-method"
}

type CaptureRateUnit int

const (
	CaptureUnitPerSecond CaptureRateUnit = iota
	CaptureUnitPerMinute
	CaptureUnitPerHour
)

type CaptureRate struct {
	Value int             `json:"value"`
	Unit  CaptureRateUnit `json:"unit"`
}

type Settings struct {
	OutputFilename string       `json:"outputFilename"`
	OutputType     OutputType   `json:"outputType"`
	OutputMethod   OutputMethod `json:"outputMethod"`

	CaptureRate CaptureRate

	WindowRect Rect `json:"windowRect"`

	HideOnCapture bool `json:"HideOnCapture"`

	FrameRate framerate.T
}

const (
	FramesPerSecond = float64(1)
	FramesPerMinute = float64(1) / 60
	FramesPerHour   = float64(1) / 60 / 60
)

// shift: step by 10
// backspace: change unit (FPS, FPM, FPH)

// [space]: take one screenshot

// [enter]: record
// rate: 1 screenshot per minute
// [shift] [-][+] [backspace]
//

func incrementFPS(fps float64) float64 {
	if fps < FramesPerHour {
		return fps + FramesPerHour/60*5
	}
	if fps-FramesPerHour < FramesPerHour {
		return fps + FramesPerHour/60
	}
	if fps-FramesPerMinute < FramesPerMinute {
		return fps + FramesPerMinute/60*5
	}
	if fps < 1 {
		return fps + FramesPerMinute/60
	}

	if fps > 5 {
		println("s5")
		return fps + 5
	}

	return fps + 1
}

func decrementFPS(fps float64) float64 {
	if fps > 5 {
		return fps - 5
	}
	if fps > 1 {
		return fps - 1
	}

	if fps > FramesPerMinute {
		return fps - FramesPerMinute
	}

	return fps - FramesPerHour
}

func getFrameDuration(fps float64) time.Duration {
	return time.Duration(1/fps*1000) * time.Millisecond
}

func getFrameStepSize(fps float64) float64 {
	//if fps > 1 {
	//	return 1
	//}
	//if fps >= FramesPerMinute {
	//	return FramesPerMinute
	//}
	//return FramesPerHour
	duration := getFrameDuration(fps)
	fmt.Printf("duration %v %v\n", duration.Hours(), duration.Minutes())
	if duration.Seconds() >= 3600 {
		println("x")
		return FramesPerMinute
	}
	if duration.Seconds() >= 60 {
		println("y")
		return FramesPerHour
	}
	println("z")
	return 1
}
