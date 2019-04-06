package main

import (
	"bytes"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mattn/go-mjpeg"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"golang.org/x/image/colornames"
	"golang.org/x/image/draw"
)

var (
	srcWidth  = flag.Int("width", 1296, "Width of src image")
	srcHeight = flag.Int("height", 768, "Height of src image")
	logDebug  = flag.Bool("debug", false, "Enable Debug log level")
	port      = flag.String("port", "8888", "Port for http server")
	urls      = flag.StringArray("url", []string{}, "List of urls to get mjpeg streams from")

	minioKey      = flag.String("minioKey", "", "Minio AccessKeyID")
	minioSecret   = flag.String("minioSecret", "", "Minio SecretAccessKey")
	minioEndpoint = flag.String("minioEndpoint", "", "Minio endpoint")
	minioBucket   = flag.String("minioBucket", "public", "Minio bucket")

	domain = flag.String("domain", "example.com", "Domain")

	snapshotCam = flag.String("snapshotCam", "", "Cam url to take screenshot from")

	filename = "html/index_template.html"
)

type ProcessedImages struct {
	srcImg, scaledImg image.Image
	nr                int
}

type Subscriptions struct {
	C chan *ProcessedImages
}

type ImgSizes struct {
	width, height, thWidth, thHeight, totalHeight, nrImages int
}

func NewImageSizes(width, height, nrStreams int) ImgSizes {
	// calculate sizes
	thumbWidth, thumbHeight := width/nrStreams, height/nrStreams
	totalHeight := height + thumbHeight
	return ImgSizes{
		width:       width,
		height:      height,
		thHeight:    thumbHeight,
		thWidth:     thumbWidth,
		nrImages:    nrStreams,
		totalHeight: totalHeight,
	}
}

func (is ImgSizes) GetThRectNr(nr int) image.Rectangle {
	return image.Rect(nr*is.thWidth,
		is.height,
		nr*is.thWidth+is.thWidth,
		is.totalHeight)
}

func main() {
	log.Infoln("Starting Up...")

	// config
	flag.Parse()
	log.SetLevel(log.InfoLevel)
	if *logDebug {
		log.SetLevel(log.DebugLevel)
	}
	if len(*urls) < 2 {
		log.Fatal("There must be at least two stream urls")
	}

	log.Infoln("URLs to get streams from:", *urls)

	sizes := NewImageSizes(*srcWidth, *srcHeight, len(*urls))

	// init streams
	var streams []*mjpeg.Stream
	for range *urls {
		streams = append(streams, mjpeg.NewStream())
	}

	buf := bytes.Buffer{}
	err := WriteHtml(filename, &buf, sizes)
	if err != nil {
		log.Fatalf("Error writing the html template: %v", err)
	}

	go func() {
		C := make(chan *ProcessedImages, sizes.nrImages)
		for i, url := range *urls {
			Subscribe(url, C, i, sizes)
		}

		images := make([]*ProcessedImages, sizes.nrImages)
		drawSign := make([]bool, sizes.nrImages)

		ti := time.NewTicker(time.Second)
		for {
			select {
			case img := <-C:
				images[img.nr] = img
			case <-ti.C:
				for _, v := range images {
					if v == nil {
						continue
					}
				}
				t := time.Now()

				wg := sync.WaitGroup{}
				for i, stream := range streams {
					wg.Add(1)
					go func(j int, stt *mjpeg.Stream) {
						defer wg.Done()

						outputImg := image.NewRGBA(image.Rect(0, 0, sizes.width, sizes.totalHeight))

						draw.Draw(outputImg, images[j].srcImg.Bounds(), images[j].srcImg, image.ZP, draw.Src)

						for i, im := range images {
							draw.Draw(outputImg, sizes.GetThRectNr(i), im.scaledImg, image.ZP, draw.Src)
						}

						if drawSign[j] {
							draw.Draw(outputImg, image.Rect(0, 0, 10, 10), image.NewUniform(colornames.Red), image.ZP, draw.Src)
							drawSign[j] = false
						} else {
							drawSign[j] = true
						}

						buf := &bytes.Buffer{}
						err := jpeg.Encode(buf, outputImg, &jpeg.Options{
							Quality: 90,
						})

						if err != nil {
							log.Println(err)
							return
						}
						stt.Update(buf.Bytes())
					}(i, stream)
				}
				wg.Wait()
				log.Debugf("All %v streams Processed in %v", sizes.nrImages, time.Since(t))
			}
		}
	}()

	for i, v := range streams {
		http.HandleFunc(fmt.Sprintf("/image/%v", i), http.HandlerFunc(v.ServeHTTP))
	}

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write(buf.Bytes())
		writer.Header().Add("content-type", "text/html")
	})

	snapshotClient, err := NewMinioClient(ClientOptions{
		Endpoint:        *minioEndpoint,
		Domain:          *domain,
		Bucket:          *minioBucket,
		SecretAccessKey: *minioSecret,
		AccessKeyID:     *minioKey,
		CamUrl:          *snapshotCam,
	})
	if err != nil {
		log.Fatalln(err)
	}

	http.Handle("/snapshot", snapshotClient)

	log.Println(http.ListenAndServe(net.JoinHostPort("", *port), nil))
}

func Subscribe(url string, processedImgChan chan *ProcessedImages, nr int, sizes ImgSizes) *Subscriptions {
	imageChan := make(chan image.Image)

	go func() {
		var srcImg image.Image
		var scaledImg draw.Image
		for {
			select {
			case srcImg = <-imageChan:
				t := time.Now()

				// rescale image
				scaledImg = image.NewRGBA(image.Rect(0, 0, sizes.thWidth, sizes.thHeight))
				draw.ApproxBiLinear.Scale(scaledImg, scaledImg.Bounds(), srcImg, srcImg.Bounds(), draw.Src, nil)

				processedImg := &ProcessedImages{
					srcImg:    srcImg,
					scaledImg: scaledImg,
					nr:        nr,
				}
				log.Debugln(url, "Scaling single image", time.Since(t))
				select {
				case processedImgChan <- processedImg:
				case <-time.After(time.Second):
					log.Println(url, "Giving up")
				}
			}
		}
	}()

	go func() {
		nrOfRetries := 0
		for {
			log.Infoln(url, "Connecting")
			imageChan <- image.NewRGBA(image.Rect(0, 0, sizes.width, sizes.totalHeight))

			d, err := mjpeg.NewDecoderFromURL(url)
			if err != nil {
				if nrOfRetries < 30 {
					nrOfRetries++
				}
				sleepDuration := time.Second * time.Duration(nrOfRetries)
				log.Errorln(url, "Error Connecting trying to sleep for", sleepDuration)
				time.Sleep(sleepDuration)
				continue
			}
			nrOfRetries = 0
			log.Infoln(url, "Connected")

			receivingChan := make(chan image.Image, 1)
			go func() {
				for {
					img, err := d.Decode()
					if err != nil {
						log.Error(url, "Error Decoding")
						close(receivingChan)
						return
					}
					receivingChan <- img
				}
			}()

		imgLoop:
			for {
				select {
				case img, ok := <-receivingChan:
					if !ok {
						log.Error(url, "Channel was closed")
						break imgLoop
					}
					imageChan <- img
				case <-time.After(time.Second * 5):
					log.Error(url, "Timeout for receiving Images")
					break imgLoop
				}
			}
		}
	}()

	return &Subscriptions{
		C: processedImgChan,
	}
}

// All things html template
type Areas struct {
	Id int
	image.Rectangle
}

type Data struct {
	Width, Height int
	Areas         []Areas
}

func WriteHtml(filename string, w io.Writer, sizes ImgSizes) error {
	templ, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	t, err := template.New("index").Parse(string(templ))
	if err != nil {
		return err
	}
	data := Data{
		Width:  sizes.width,
		Height: sizes.totalHeight,
	}

	for i := 0; i < sizes.nrImages; i++ {
		data.Areas = append(data.Areas, Areas{
			Id:        i,
			Rectangle: sizes.GetThRectNr(i),
		})
	}
	return t.Execute(w, data)
}
