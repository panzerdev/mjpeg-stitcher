package main

import (
	"context"
	"github.com/mattn/go-mjpeg"
	"image"
	"log"
	"time"
)

func SubscribeToMjpgStream(ctx context.Context, url string) (chan image.Image, error) {
	decoder, err := mjpeg.NewDecoderFromURL(url)
	if err != nil {
		return nil, err
	}

	c := make(chan image.Image)
	go func() {
		for {
			img, err := decoder.Decode()
			if err != nil {
				log.Println("Stream err", err)
				close(c)
				return
			}
			select {
			case c <- img:
			case <-time.After(time.Second):
			case <-ctx.Done():
				close(c)
			}
		}
	}()
	return c, nil
}
