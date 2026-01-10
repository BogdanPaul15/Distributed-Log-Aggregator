package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"log-consumer/internal/kafka"
	"log-consumer/internal/storage"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Starting metrics server on :2112")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaTopic := getEnv("KAFKA_TOPIC", "logs")
	kafkaGroupID := getEnv("KAFKA_GROUP_ID", "log-consumer-group")
	opensearchAddr := getEnv("OPENSEARCH_ADDR", "https://localhost:9200")

	brokers := strings.Split(kafkaBrokers, ",")
	opensearchAddrs := strings.Split(opensearchAddr, ",")

	osClient, err := storage.NewOpenSearchClient(opensearchAddrs)
	if err != nil {
		log.Fatalf("Failed to create OpenSearch client: %v", err)
	}

	consumer := kafka.NewConsumer(brokers, kafkaTopic, kafkaGroupID, osClient)

	if err := consumer.Start(context.Background()); err != nil {
		log.Fatalf("Consumer failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
