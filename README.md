# mjpeg-stitcher
Service to combine multiple mjpeg streams to overview streams in pure Go.

![Example of overview generated](https://raw.githubusercontent.com/panzerdev/mjpeg-stitcher/master/img/overview_example.png)

This service is taking 2 to n streams with the same image sizes and combines all streams to overview streams where a different stream is full size and the rest are thubnails.

By clicking on the thubnails the stream can be switched.

# Building
Go _1.11_ is the minimum version due to the use of Modules. This repository can be checked out anywhere outside the `GOPATH` and build with `go build`. 

For the Raspberry Pi 2 or 3(b+) `env GOOS=linux GOARCH=arm GOARM=7 go build -o mjpeg`

# Running
```
Usage of ./mjpeg-stitcher:
      --debug             Enable Debug log level
      --height int        Height of src image (default 768)
      --port string       Port for http server (default "8888")
      --url stringArray   List of urls to get mjpeg streams from
      --width int         Width of src image (default 1296)
```


`./mjpeg-stitcher --url http://cam1.stream --url http://cam2.stream`

IMPORTATNT: The html folder needs to be in the running directory of the binary