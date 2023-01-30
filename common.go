package main

import (
	"fmt"
	"image"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
)

type Void struct{}

func TrimExt(filename string) (baseFilename, ext string) {
	ext = filepath.Ext(filename)
	baseFilename = strings.TrimSuffix(filename, ext)
	return
}

func NextLatestIncrementedFilename(filename string) string {
	baseFilename, _, ext := parseIncrementFilename(filename)
	files, err := filepath.Glob(baseFilename + "*")
	if err != nil {
		panic(err)
	}

	maxNum := 0
	for _, file := range files {
		_, num, ext2 := parseIncrementFilename(file)
		if num > maxNum && ext == ext2 {
			maxNum = num
		}
	}

	maxNum++

	return fmt.Sprintf("%v-%v%v", baseFilename, maxNum, ext)
}

func parseIncrementFilename(filename string) (base string, num int, ext string) {
	fileExt := filepath.Ext(filename)
	filename = strings.TrimSuffix(filename, fileExt)

	if filename == "" && fileExt != "" {
		filename, fileExt = fileExt, ""
	}

	i := len(filename) - 1
	if i < 0 {
		return "", 0, ""
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

	if filename[len(filename)-1] == '-' {
		filename = filename[0 : len(filename)-1]
	}

	if n, err := strconv.Atoi(digits); err == nil {
		currentNum = n
	}

	return filename, currentNum, fileExt
}

func IncrementFilename(filename string) string {
	filename, num, ext := parseIncrementFilename(filename)
	if filename == "" && ext == "" {
		return ""
	}
	num++
	return fmt.Sprintf("%v-%v%v", filename, num, ext)
}

func GetWindowBounds() image.Rectangle {
	x, y := ebiten.WindowPosition()
	w, h := ebiten.WindowSize()
	return image.Rect(x+3, y+3, x+w-3, y+h-3)
}

func TimeRun(label string, fn func()) {
	now := time.Now()
	fn()
	elapsed := int(time.Since(now).Milliseconds())
	println(label, elapsed)
}

type Queue[T any] struct {
	data  []T
	index int
	mu    sync.Mutex
}

func CreateQueue[T any](initSize int) Queue[T] {
	return Queue[T]{
		data:  make([]T, initSize),
		index: 0,
	}
}

func (q *Queue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.index >= len(q.data) {
		q.data = growSlice(q.data)
		if q.index >= len(q.data) {
			panic("failed to sufficiently grow slice")
		}
	}

	q.data[q.index] = item
	q.index++
}

func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.index == 0 {
		var none T
		return none, false
	}
	q.index--
	return q.data[q.index], true
}

func (q *Queue[T]) IsEmpty() bool {
	return q.index == 0 || len(q.data) == 0
}

func growSlice[T any](slice []T) []T {
	newSize := cap(slice) * 2
	resized := make([]T, newSize)
	copy(resized, slice)
	return resized
}

type Task[T any] struct {
	Result T
	Error  error
	done   atomic.Bool
}

func (task *Task[T]) Finish()      { task.done.Store(true) }
func (task *Task[T]) IsDone() bool { return task.done.Load() }
