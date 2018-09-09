package main

import (
	"github.com/gobuffalo/packr"
	"os"
	"path"
	"testing"
)

func TestWriteHtml(t *testing.T) {
	tmpBox := packr.NewBox(boxFolder)
	f, err := os.Create(path.Join("test", "test.html"))
	if err != nil{
		t.Fatal(err)
	}
	defer f.Close()

	sizes := NewImageSizes(1200, 720, 5)

	err = WriteHtml(&tmpBox, f, sizes)
	if err != nil{
		t.Fatal(err)
	}
}
