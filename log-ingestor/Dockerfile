FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o log-ingestor cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/log-ingestor .

EXPOSE 8080

CMD ["./log-ingestor"]
