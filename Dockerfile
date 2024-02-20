FROM golang:1.22

RUN apt-get update && apt-get install -y \
    libwebp-dev \
    libpng-dev \
    libjpeg-dev \
    libaom-dev

WORKDIR /go/src/app
COPY . .

RUN go build -o app

EXPOSE 8080

CMD ["./app"]
