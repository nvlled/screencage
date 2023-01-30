package main

import "testing"

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

func TestLatestIncrementedFile(t *testing.T) {
	result := NextLatestIncrementedFilename("/home/nvlled/screen-1.gif")
	println(result)
}

func TestQueue(t *testing.T) {
	q := CreateQueue[int](10)

	_, ok := q.Pop()
	if ok {
		t.Error("cannot pop from empty queue")
	}
	q.Push(123)
	if val, ok := q.Pop(); !ok || val != 123 {
		t.Errorf("failed to retrieved pushed value: %v", val)
	}

	for i := 0; i < 15; i++ {
		q.Push(i)
	}

	for i := 14; i >= 0; i-- {
		if val, ok := q.Pop(); !ok || i != val {
			t.Errorf("failed to retrieved pushed value: %v", val)
		}
	}

}
