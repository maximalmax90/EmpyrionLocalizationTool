package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	localConfigDirName = ".elt-local"
	remoteConfigFile   = "remote.json"
	authorConfigFile   = "author.json"
	remoteName         = "origin"
)

type RemoteConfig struct {
	URL string `json:"url"`

	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`

	Push string `json:"push"`
}

func (cfg RemoteConfig) auth() *githttp.BasicAuth {
	if cfg.Token == "" {
		return nil
	}
	username := cfg.Username
	if username == "" {
		username = "token"
	}
	return &githttp.BasicAuth{Username: username, Password: cfg.Token}
}

func (s *Store) localConfigDir() string   { return filepath.Join(s.dir, localConfigDirName) }
func (s *Store) remoteConfigPath() string { return filepath.Join(s.localConfigDir(), remoteConfigFile) }
func (s *Store) authorConfigPath() string { return filepath.Join(s.localConfigDir(), authorConfigFile) }

type AuthorConfig struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Store) SetAuthor(cfg AuthorConfig) error {
	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.authorConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("store: writing author config: %w", err)
	}
	return nil
}

func (s *Store) Author() (AuthorConfig, bool, error) {
	data, err := os.ReadFile(s.authorConfigPath())
	if os.IsNotExist(err) {
		return AuthorConfig{}, false, nil
	}
	if err != nil {
		return AuthorConfig{}, false, err
	}
	var cfg AuthorConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AuthorConfig{}, false, fmt.Errorf("store: %s is not valid: %w", authorConfigFile, err)
	}
	return cfg, true, nil
}

func (s *Store) commitSignature(override *AuthorConfig) *object.Signature {
	if override != nil && override.Name != "" && override.Email != "" {
		return &object.Signature{Name: override.Name, Email: override.Email, When: time.Now()}
	}
	if cfg, ok, err := s.Author(); err == nil && ok && cfg.Name != "" && cfg.Email != "" {
		return &object.Signature{Name: cfg.Name, Email: cfg.Email, When: time.Now()}
	}
	return &object.Signature{Name: "elt", Email: "elt@localhost", When: time.Now()}
}

func (s *Store) SetRemote(cfg RemoteConfig) error {
	if cfg.URL == "" {
		return fmt.Errorf("store: remote URL is required")
	}
	if cfg.Push == "" {
		cfg.Push = "manual"
	}

	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.remoteConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("store: writing remote config: %w", err)
	}

	if err := s.repo.DeleteRemote(remoteName); err != nil && !errors.Is(err, git.ErrRemoteNotFound) {
		return fmt.Errorf("store: removing previous remote: %w", err)
	}
	if _, err := s.repo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{cfg.URL}}); err != nil {
		return fmt.Errorf("store: configuring remote: %w", err)
	}
	return nil
}

func (s *Store) Remote() (RemoteConfig, bool, error) {
	data, err := os.ReadFile(s.remoteConfigPath())
	if os.IsNotExist(err) {
		return RemoteConfig{}, false, nil
	}
	if err != nil {
		return RemoteConfig{}, false, err
	}
	var cfg RemoteConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RemoteConfig{}, false, fmt.Errorf("store: %s is not valid: %w", remoteConfigFile, err)
	}
	return cfg, true, nil
}

func (s *Store) Push() error {
	cfg, ok, err := s.Remote()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("store: no remote configured; call SetRemote first")
	}
	return s.pushTo(cfg)
}

func (s *Store) pushTo(cfg RemoteConfig) error {
	remote, err := s.repo.Remote(remoteName)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	err = remote.Push(&git.PushOptions{Auth: cfg.auth()})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("store: pushing to %s: %w", cfg.URL, err)
	}
	return nil
}

func CloneFrom(dir string, cfg RemoteConfig) (*Store, error) {
	s, err := CloneRaw(dir, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, manifestFileName)); err != nil {
		return nil, fmt.Errorf("store: cloned %s but it does not look like a project store: %w", cfg.URL, err)
	}
	return s, nil
}

func CloneRaw(dir string, cfg RemoteConfig) (*Store, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("store: remote URL is required")
	}
	scan, err := scanDir(dir)
	if err != nil {
		return nil, fmt.Errorf("store: checking %s: %w", dir, err)
	}
	if scan.hasGit {
		return nil, fmt.Errorf("store: %s already has its own git repository — clone needs a location with no existing git history (point at an empty directory, or open it directly instead)", dir)
	}
	if scan.hasManifest || scan.hasOther {
		return nil, fmt.Errorf("store: %s already contains files that don't look right to clone into (a README alone is fine)", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: creating %s: %w", dir, err)
	}

	repo, err := git.PlainClone(dir, false, &git.CloneOptions{URL: cfg.URL, Auth: cfg.auth()})
	if err != nil {
		return nil, fmt.Errorf("store: cloning %s: %w", cfg.URL, err)
	}
	s := &Store{dir: dir, repo: repo}
	if err := s.SetRemote(cfg); err != nil {
		return nil, err
	}
	return s, nil
}
