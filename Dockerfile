FROM golang:1.24

RUN apt-get update && apt-get install -y libvips-dev

WORKDIR /go/src/app

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код
COPY . .

RUN go build -o app

EXPOSE 8080

CMD ["./app"]
