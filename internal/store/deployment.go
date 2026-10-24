package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const deploymentConfigFile = "deployment.json"

func (s *Store) deploymentConfigPath() string {
	return filepath.Join(s.localConfigDir(), deploymentConfigFile)
}

type TLSConfig struct {
	Enabled bool   `json:"enabled"`
	Cert    string `json:"cert,omitempty"`
	Key     string `json:"key,omitempty"`
}

type DeploymentConfig struct {
	Listen    string     `json:"listen,omitempty"`
	MaxSyncMB int        `json:"max_sync_mb,omitempty"`
	TLS       *TLSConfig `json:"tls,omitempty"`
}

func (s *Store) SetDeployment(cfg DeploymentConfig) error {
	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.deploymentConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("store: writing deployment config: %w", err)
	}
	return nil
}

func (s *Store) Deployment() (DeploymentConfig, bool, error) {
	data, err := os.ReadFile(s.deploymentConfigPath())
	if os.IsNotExist(err) {
		return DeploymentConfig{}, false, nil
	}
	if err != nil {
		return DeploymentConfig{}, false, err
	}
	var cfg DeploymentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DeploymentConfig{}, false, fmt.Errorf("store: %s is not valid: %w", deploymentConfigFile, err)
	}
	return cfg, true, nil
}
