package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"elt/internal/ai"
)

const aiConfigFile = "ai.json"

func (s *Store) aiConfigPath() string { return filepath.Join(s.localConfigDir(), aiConfigFile) }

func (s *Store) SetAIConfig(cfg ai.Config) error {
	if cfg.URL == "" {
		return fmt.Errorf("store: AI provider URL is required")
	}
	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.aiConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("store: writing AI config: %w", err)
	}
	return nil
}

func (s *Store) AIConfig() (ai.Config, bool, error) {
	data, err := os.ReadFile(s.aiConfigPath())
	if os.IsNotExist(err) {
		return ai.Config{}, false, nil
	}
	if err != nil {
		return ai.Config{}, false, err
	}
	var cfg ai.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ai.Config{}, false, fmt.Errorf("store: %s is not valid: %w", aiConfigFile, err)
	}
	return cfg, true, nil
}
