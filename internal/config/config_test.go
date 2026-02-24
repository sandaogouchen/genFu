package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadConfig(t *testing.T) {
	path, err := configPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.LLM.Endpoint == "" || cfg.PG.DSN == "" {
		t.Fatalf("missing config values")
	}
}

func TestNormalizeInvalidTimeout(t *testing.T) {
	path, err := configPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.LLM.Timeout = "bad"
	normalized, err := normalize(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized.LLM.Timeout != 0 {
		t.Fatalf("expected zero timeout")
	}
}

func configPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(wd, "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", errors.New("config_yaml_not_found")
		}
		wd = parent
	}
}
