package testutil

import (
	"testing"

	"genFu/internal/config"
)

func LoadConfig(t *testing.T) config.NormalizedConfig {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}
