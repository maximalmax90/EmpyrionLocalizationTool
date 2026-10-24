package localai

import (
	"encoding/json"
	"os"
	"path/filepath"

	"elt/internal/ai"
)

func configFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "elt", "ai.json"), nil
}

func LoadConfig() (ai.Config, error) {
	path, err := configFilePath()
	if err != nil {
		return ai.Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ai.Config{}, nil
		}
		return ai.Config{}, err
	}
	var cfg ai.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ai.Config{}, err
	}
	return cfg, nil
}

func SaveConfig(cfg ai.Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Configured(cfg ai.Config) bool {
	return cfg.URL != ""
}
