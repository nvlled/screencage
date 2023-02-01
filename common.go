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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/nvlled/carrot"
)

type Void struct{}

func TrimExt(filename string) (baseFilename, ext string) {
	ext = filepath.Ext(filename)
	baseFilename = strings.TrimSuffix(filename, ext)
	return
}

func ReplaceIncrementedFilename(filename string, counter int) string {
	baseFilename, _, ext := parseIncrementFilename(filename)
	return fmt.Sprintf("%v-%v%v", baseFilename, counter, ext)
}

func NextLatestIncrementedFilename(filename string) (string, int) {
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

	return fmt.Sprintf("%v-%v%v", baseFilename, maxNum, ext), maxNum
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

// FIFO not FILO
// queues are FIFO?
// doesn't matter, what kind of list do I want here?
// okay, I'm dumb. I'm actually thinking of a collection
// that behaves like push/pop in js, which is LIFO,
// but what I want here is FIFO.
// I hear a suppressed voice in my head, whispering
// "just go-get a goshdarn library for that"
// but no, this one is trivial to implement.
type Queue[T any] struct {
	data      []T
	popIndex  int
	pushIndex int
	mu        sync.Mutex

	defaultValue T
}

func CreateQueue[T any](initSize int) Queue[T] {
	return Queue[T]{
		data:      make([]T, initSize),
		pushIndex: 0,
		popIndex:  0,
	}
}

// |-1 0  1  2  3  4  5
// |   a  b  c  d  e  f
// |   xy               size=0
// |   x  y             size=1
// |   x           y    size=4
// |   x              y size=5
//
// |      y  x          size=5 N-x+y 6-2+1
// |   y              x size=1       6-5+0
func (q *Queue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	nextPushIndex := (q.pushIndex + 1) % len(q.data)

	size := q.Size()
	if size >= len(q.data) || nextPushIndex == q.popIndex {
		q.data = growSlice(q.data, q.popIndex)
		if q.pushIndex >= len(q.data) {
			panic("failed to sufficiently grow slice")
		}
		q.popIndex = 0
		q.pushIndex = size
	}

	q.data[q.pushIndex] = item
	q.pushIndex++
	if q.pushIndex >= len(q.data) {
		q.pushIndex = 0
	}
}

func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.IsEmpty() {
		var none T
		return none, false
	}

	value := q.data[q.popIndex]
	q.data[q.popIndex] = q.defaultValue
	q.popIndex++

	if q.popIndex >= len(q.data) {
		q.popIndex = 0
	}

	return value, true
}

func (q *Queue[T]) Size() int {
	if q.popIndex == q.pushIndex {
		return 0
	}
	if q.popIndex < q.pushIndex {
		return q.pushIndex - q.popIndex
	}
	return len(q.data) - q.popIndex + q.pushIndex
}

func (q *Queue[T]) IsEmpty() bool {
	return q.pushIndex == q.popIndex || q.popIndex >= len(q.data)
}

func growSlice[T any](slice []T, startIndex int) []T {
	capacity := cap(slice)
	newSize := (capacity + 1) * 2
	resized := make([]T, newSize)
	for i := 0; i < capacity; i++ {
		resized[i] = slice[(i+startIndex)%capacity]
	}
	return resized
}

type Task[T any] struct {
	Result T
	Err    error
	done   atomic.Bool
}

func (task *Task[T]) Finish()      { task.done.Store(true) }
func (task *Task[T]) IsDone() bool { return task.done.Load() }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func awaitEnter(ctrl *carrot.Control) {
	ctrl.Yield()
	ctrl.YieldUntil(func() bool {
		return inpututil.IsKeyJustPressed(ebiten.KeyEnter)
	})
}

func awaitNextDraw(ctrl *carrot.Control, lasttDraw *int64) {
	prevLastDraw := *lasttDraw
	for {
		ctrl.Yield()
		if prevLastDraw < *lasttDraw {
			break
		}
	}
	ctrl.Yield()
}
