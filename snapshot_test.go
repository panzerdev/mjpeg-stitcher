package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mattn/go-mjpeg"
	"github.com/minio/minio-go"
	"github.com/stretchr/testify/assert"
	"image"
	"image/jpeg"
	"io"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeMinio struct {
}

func (fakeMinio) PutObject(bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (n int64, err error) {
	return 1, nil
}

func TestGetSnapshotUrl(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := mjpeg.NewStream()

	server := httptest.NewServer(stream)
	defer server.Close()

	client := &SnapshotClient{
		StorageClient: fakeMinio{},
		targetCamUrl:  server.URL,
		domain:        "test.zone",
		bucket:        "test",
		cacheChan:     make(chan image.Image),
	}

	go client.imagePreCache(ctx)
	//go func() {client.cacheChan <- image.NewGray(image.Rect(0, 0, 100, 100))}()
	go func() {
		for {
			uniformImage := image.NewGray(image.Rect(0, 0, 1000, 1000))
			buf := bytes.Buffer{}
			err := jpeg.Encode(&buf, uniformImage, &jpeg.Options{Quality: 100})
			assert.NoError(t, err)
			err = stream.Update(buf.Bytes())
			assert.NoError(t, err)

			select {
			case <-ctx.Done():
				err := stream.Close()
				assert.NoError(t, err)
				return
			case <-time.After(time.Millisecond * 100):
			}
		}
	}()

	url, err := client.GetSnapshotUrl()
	assert.NoError(t, err)
	cancel()
	assert.Regexp(t, fmt.Sprintf("https://%v.%v/%v/snapshots/.*/snapshot.jpg", client.bucket, client.domain, client.bucket), url)
}
