FROM golang:1.12.0 as build

ENV CGO_ENABLED=0

COPY go.mod go.sum /app/

WORKDIR /app

RUN go mod download

COPY . /app

RUN go build -o mjpeg

FROM alpine:3.9

RUN apk update \
        && apk add ca-certificates tzdata

WORKDIR app

COPY --from=build /app/mjpeg mjpeg
# copy assets
COPY html html/

ENTRYPOINT ["./mjpeg"]
