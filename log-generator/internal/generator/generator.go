package generator

import "log-generator/internal/model"

type Generator interface {
	Generate() model.LogEvent
}
