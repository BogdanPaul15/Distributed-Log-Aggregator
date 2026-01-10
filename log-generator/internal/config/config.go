package config

import (
	"log-generator/internal/engine"
	"log-generator/internal/generator/random"
	"log-generator/internal/storage/http"
)

type StorageType string

const (
	StorageConsole StorageType = "console"
	StorageHTTP    StorageType = "http"
	StorageFile    StorageType = "file"
)

type StorageConfig struct {
	Type StorageType     `yaml:"type"`
	HTTP http.HTTPConfig `yaml:"http"`
}

type Config struct {
	Engine    engine.EngineConfig    `yaml:"engine"`
	Storage   StorageConfig          `yaml:"storage"`
	Generator random.GeneratorConfig `yaml:"generator"`
}
