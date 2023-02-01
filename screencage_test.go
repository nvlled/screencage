package screencage

import (
	"testing"
)

func TestIncrementFilename(t *testing.T) {
	for _, entry := range [][]string{
		{"filename.png", "filename-1.png"},
		{"filename-1.png", "filename-2.png"},
		{"filename-.png", "filename-1.png"},
		{"filename-x.png", "filename-x-1.png"},
		{"", ""},
		{".file", ".file-1"},
		{"-.file", "-1.file"},
		{"/home/nvlled/screen-1.gif", "/home/nvlled/screen-2.gif"},
		{"filename", "filename-1"},
		{"filename-1", "filename-2"},
	} {
		expected := entry[1]
		actual := IncrementFilename(entry[0])
		if actual != expected {
			t.Errorf("expected: %v | got %v", expected, actual)
		}
	}
}

func TestQueueSizes(t *testing.T) {
	q := CreateQueue[int](3)
	q.Push(1)
	if q.Size() != 1 {
		t.Errorf("wrong size, expected=%v, got=%v", 1, q.Size())
	}
	q.Push(2)
	if q.Size() != 2 {
		t.Errorf("wrong size, expected=%v, got=%v", 2, q.Size())
	}
	q.Push(3)
	if q.Size() != 3 {
		t.Errorf("wrong size, expected=%v, got=%v", 3, q.Size())
	}

	q = CreateQueue[int](3)
	q.Push(1)
	q.Pop()
	if q.Size() != 0 {
		t.Errorf("wrong size, expected=%v, got=%v", 3, q.Size())
	}
	q.Push(1)
	q.Push(2)
	if q.Size() != 2 {
		t.Errorf("wrong size, expected=%v, got=%v", 2, q.Size())
	}
}

func TestQueue(t *testing.T) {
	q := CreateQueue[int](10)

	_, ok := q.Pop()
	if ok {
		t.Error("cannot pop from empty queue")
	}

	q.Push(123)
	if q.Size() != 1 {
		t.Errorf("wrong size")
	}

	if val, ok := q.Pop(); !ok || val != 123 {
		t.Errorf("failed to retrieved pushed value: %v", val)
	}

	q.Push(1)
	q.Push(2)
	q.Push(3)

	if q.Size() != 3 {
		t.Errorf("wrong size")
	}

	val, ok := q.Pop()
	if !ok || val != 1 {
		t.Errorf("wrong value, expected=%v, got=%v", 1, val)
	}
	val, ok = q.Pop()
	if !ok || val != 2 {
		t.Errorf("wrong value, expected=%v, got=%v", 2, val)
	}
	val, ok = q.Pop()
	if !ok || val != 3 {
		t.Errorf("wrong value, expected=%v, got=%v", 3, val)
	}

	if !q.IsEmpty() {
		t.Errorf("must not be empty")
	}

	for i := 0; i < 15; i++ {
		q.Push(i)
	}
	if q.Size() != 15 {
		t.Errorf("wrong size %v", q.Size())
	}

	for i := 0; i < 15; i++ {
		if val, ok := q.Pop(); !ok || i != val {
			t.Errorf("failed to retrieved pushed value: expected=%v, got=%v", i, val)
		}
	}

	if q.Size() != 0 {
		t.Errorf("wrong size")
	}
}
