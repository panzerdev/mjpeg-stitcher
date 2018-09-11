package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewImageSizes(t *testing.T) {
	w, h, nr := 1000, 1000, 10
	sizes := NewImageSizes(w, h, nr)
	a := assert.New(t)
	a.Equal(w, sizes.width)
	a.Equal(h, sizes.height)
	a.Equal(nr, sizes.nrImages)
	a.Equal(h/nr, sizes.thHeight)
	a.Equal(w/nr, sizes.thWidth)
	a.Equal(h+h/nr, sizes.totalHeight)

	for i := 0; i < nr; i++ {
		rect := sizes.GetThRectNr(i)
		a.Equal(rect.Min.X, i*sizes.thWidth)
		a.Equal(rect.Max.X, i*sizes.thWidth+sizes.thWidth)
		a.Equal(rect.Min.Y, sizes.height)
		a.Equal(rect.Max.Y, sizes.totalHeight)
	}
}
