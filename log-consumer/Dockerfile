FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o log-consumer cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/log-consumer .

CMD ["./log-consumer"]
