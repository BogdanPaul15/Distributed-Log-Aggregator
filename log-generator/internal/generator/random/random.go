package random

import (
	"fmt"
	"maps"
	"math/rand"
	"sync"
	"time"

	"log-generator/internal/model"
)

type ServiceConfig struct {
	Messages     map[model.LogLevel][]string `yaml:"messages"`
	StaticFields map[string]any              `yaml:"static_fields"`
}

type GeneratorConfig struct {
	Weights        map[model.LogLevel]int   `yaml:"weights"`
	Services       []string                 `yaml:"services"`
	ServiceConfig  map[string]ServiceConfig `yaml:"service_profiles"`
	GlobalMetadata map[string]any           `yaml:"global_metadata"`
}

type RandomGenerator struct {
	mu             sync.RWMutex
	weights        map[model.LogLevel]int
	services       []string
	serviceConfig  map[string]ServiceConfig
	globalMetadata map[string]any
}

func NewRandomGenerator(cfg GeneratorConfig) *RandomGenerator {
	return &RandomGenerator{
		weights:        cfg.Weights,
		services:       cfg.Services,
		serviceConfig:  cfg.ServiceConfig,
		globalMetadata: cfg.GlobalMetadata,
	}
}

func (rg *RandomGenerator) SetWeights(weights map[model.LogLevel]int) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.weights = weights
}

func (rg *RandomGenerator) calculateTotalWeight() int {
	sum := 0

	for _, weight := range rg.weights {
		sum += weight
	}

	return sum
}

func (rg *RandomGenerator) pickLevel() model.LogLevel {
	weightsSum := rg.calculateTotalWeight()

	randomPick := rand.Intn(weightsSum)

	currentSum := 0
	for level, weight := range rg.weights {
		currentSum += weight
		if randomPick < currentSum {
			return level
		}
	}

	return model.INFO
}

func (rg *RandomGenerator) pickMessage(service string, level model.LogLevel) string {
	if config, ok := rg.serviceConfig[service]; ok {
		if messages, ok := config.Messages[level]; ok && len(messages) > 0 {
			return messages[rand.Intn(len(messages))]
		}
	}

	return fmt.Sprintf("Default %s message for %s", level, service)
}

func (rg *RandomGenerator) pickPayload(service string) map[string]any {
	payload := make(map[string]any)

	maps.Copy(payload, rg.globalMetadata)

	if profile, ok := rg.serviceConfig[service]; ok {
		maps.Copy(payload, profile.StaticFields)
	}

	return payload
}

func (rg *RandomGenerator) Generate() model.LogEvent {
	rg.mu.Lock()
	level := rg.pickLevel()
	rg.mu.Unlock()

	service := rg.services[rand.Intn(len(rg.services))]
	message := rg.pickMessage(service, level)
	traceID := fmt.Sprintf("%x-%x-%x-%x", rand.Uint32(), rand.Uint32(), rand.Uint32(), rand.Uint32())
	payload := rg.pickPayload(service)

	return model.LogEvent{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Service:   service,
		TraceID:   traceID,
		Message:   message,
		Payload:   payload,
	}
}
