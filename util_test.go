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
	} {
		expected := entry[1]
		actual := IncrementFilename(entry[0])
		if actual != expected {
			t.Errorf("expected: %v | got %v", expected, actual)
		}
	}
}
