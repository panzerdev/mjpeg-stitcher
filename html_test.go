package main

import (
	"os"
	"path"
	"testing"
)

func TestWriteHtml(t *testing.T) {
	f, err := os.Create(path.Join("test", "test.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sizes := NewImageSizes(1200, 720, 5)

	err = WriteHtml(filename, f, sizes)
	if err != nil {
		t.Fatal(err)
	}
}
