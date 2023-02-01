package screencage

import "sync"

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
