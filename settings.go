package main

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

const defaultFPS = 5

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

	FPS uint8 `json:"FPS"`
}
