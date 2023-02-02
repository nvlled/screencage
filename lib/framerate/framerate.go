package framerate

import (
	"fmt"
	"time"
)

type Unit uint8

const (
	UnitSecond = iota
	UnitMinute
	UnitHour

	Unit_End
)

type T struct {
	Value int  `json:"value"`
	Unit  Unit `json:"unit"`
}

func (rate *T) String() string {
	unitStr := "seconds"
	switch rate.Unit {
	case UnitSecond:
		unitStr = "seconds"
	case UnitMinute:
		unitStr = "minutes"
	case UnitHour:
		unitStr = "hours"
	}

	return fmt.Sprintf("%v frames per %v", rate.Value, unitStr)
}

func (rate *T) Increment()           { rate.Value++ }
func (rate *T) Decrement()           { rate.Value-- }
func (rate *T) IncrementBy(step int) { rate.Value += step }
func (rate *T) DecrementBy(step int) { rate.Value -= step }

func (rate *T) Duration() time.Duration {
	const one = float64(1)
	value := float64(rate.Value)
	switch rate.Unit {
	case UnitSecond:
		return time.Duration(one/value*1000*1000) * time.Microsecond
	case UnitMinute:
		return time.Duration((one/(one/60*value))*1000*1000) * time.Microsecond
	case UnitHour:
		return time.Duration((one/(one/60/60*value))*1000*1000) * time.Microsecond
	}
	return 0
}

func (rate *T) Clamp(min, max int) {
	if rate.Value < min {
		rate.Value = min
	} else if rate.Value > max {
		rate.Value = max
	}
}
