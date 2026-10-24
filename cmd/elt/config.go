package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type AppConfig struct {
	DataDir   string `json:"data_dir,omitempty"`
	NoBrowser bool   `json:"no_browser,omitempty"`
}

func defaultConfigPath(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "elt", name+".json"), nil
}

func loadOrInitConfig(path string, defaults AppConfig) (AppConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := writeConfig(path, defaults); err != nil {
			return AppConfig{}, err
		}
		return defaults, nil
	}
	if err != nil {
		return AppConfig{}, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}

	if cfg.DataDir == "" && defaults.DataDir != "" {
		cfg.DataDir = defaults.DataDir
		if err := writeConfig(path, cfg); err != nil {
			return AppConfig{}, err
		}
	}
	return cfg, nil
}

func updateConfig(path string, defaults AppConfig, mutate func(*AppConfig)) error {
	cfg, err := loadOrInitConfig(path, defaults)
	if err != nil {
		return err
	}
	mutate(&cfg)
	return writeConfig(path, cfg)
}

func writeConfig(path string, cfg AppConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resolveSetting(cliValue, persistedValue, def string) string {
	if cliValue != "" {
		return cliValue
	}
	if persistedValue != "" {
		return persistedValue
	}
	return def
}

func resolveSettingInt(cliValue, persistedValue, def int) int {
	if cliValue != 0 {
		return cliValue
	}
	if persistedValue != 0 {
		return persistedValue
	}
	return def
}
