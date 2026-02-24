package testutil

import (
	"errors"
	"os"
	"path/filepath"
)

func ConfigPath() (string, error) {
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
