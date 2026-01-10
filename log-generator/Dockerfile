FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o log-generator cmd/log-generator/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/log-generator .

COPY config.yaml .

EXPOSE 8081

CMD ["./log-generator", "-config=config.yaml"]