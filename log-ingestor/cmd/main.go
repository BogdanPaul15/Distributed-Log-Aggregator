package main

import (
	"log"
	"os"
	"strings"

	"log-ingestor/internal/api"
	"log-ingestor/internal/kafka"
)

func main() {
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaTopic := getEnv("KAFKA_TOPIC", "logs")
	serverAddr := getEnv("SERVER_ADDR", ":8080")

	brokers := strings.Split(kafkaBrokers, ",")

	producer := kafka.NewProducer(brokers, kafkaTopic)
	defer producer.Close()

	server := api.NewServer(producer)

	log.Printf("Starting server on %s", serverAddr)
	if err := server.ListenAndServe(serverAddr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
