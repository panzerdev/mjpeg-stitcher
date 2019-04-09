package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

type ClientOptions struct {
	Endpoint                     string
	AccessKeyID, SecretAccessKey string
	Bucket                       string
	Domain                       string
	CamUrl                       string
}

type StorageClient interface {
	PutObject(bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (n int64, err error)
}

type SnapshotClient struct {
	StorageClient
	bucket, domain string
	targetCamUrl   string
	cacheChan      chan image.Image
}

func NewMinioClient(opt ClientOptions) (*SnapshotClient, error) {
	m, err := minio.New(opt.Endpoint, opt.AccessKeyID, opt.SecretAccessKey, true)
	if err != nil {
		return nil, err
	}

	sc := &SnapshotClient{
		StorageClient: m,
		bucket:        opt.Bucket,
		domain:        opt.Domain,
		targetCamUrl:  opt.CamUrl,
		cacheChan:     make(chan image.Image),
	}
	go sc.imagePreCache(context.Background())
	return sc, nil
}

func (sc *SnapshotClient) imagePreCache(ctx context.Context) {
	defer log.Infoln("imagePreCache exited")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := SubscribeToMjpgStream(ctx, sc.targetCamUrl)
		if err != nil {
			log.Println("Error Connecting feed", err)
			continue
		}

		img := <-stream
	DECODE:
		for {
			select {
			case i, ok := <-stream:
				if !ok {
					break DECODE
				}
				img = i
			case sc.cacheChan <- img:
			}
		}
	}
}

func (sc *SnapshotClient) GetSnapshotUrl() (string, error) {
	buf := bytes.Buffer{}
	err := WriteSnapshotJpg(&buf, sc)
	if err != nil {
		return "", err
	}

	generatedUuid := uuid.NewV4().String()

	path := filepath.Join("snapshots", strings.ReplaceAll(generatedUuid, "-", string(filepath.Separator)), "snapshot.jpg")

	err = sc.UploadSnapshot(&buf, int64(buf.Len()), path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%v.%v/%v/%v", sc.bucket, sc.domain, sc.bucket, path), nil
}

func WriteSnapshotJpg(w io.Writer, sc *SnapshotClient) error {
	select {
	case img := <-sc.cacheChan:
		return jpeg.Encode(w, img, &jpeg.Options{
			Quality: 90,
		})
	case <-time.After(time.Second * 10):
		return errors.New("timeout")
	}

}

func (sc *SnapshotClient) UploadSnapshot(r io.Reader, size int64, path string) error {
	_, err := sc.PutObject(sc.bucket, path, r, size, minio.PutObjectOptions{
		ContentType: "image/jpeg",
	})
	return err
}

func (sc *SnapshotClient) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	url, err := sc.GetSnapshotUrl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Infof("SnapshotUrl: %v", url)

	data := map[string]string{
		"url": url,
	}

	body, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(body)
}
