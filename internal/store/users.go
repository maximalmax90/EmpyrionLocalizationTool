package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const usersConfigFile = "users.json"

type UserConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	TokenHash string `json:"token_hash"`

	IsAdmin bool `json:"is_admin,omitempty"`
}

func (s *Store) usersConfigPath() string { return filepath.Join(s.localConfigDir(), usersConfigFile) }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("store: generating token: %w", err)
	}
	return "elt_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Store) readUsers() ([]UserConfig, error) {
	data, err := os.ReadFile(s.usersConfigPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var users []UserConfig
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("store: %s is not valid: %w", usersConfigFile, err)
	}
	return users, nil
}

func (s *Store) writeUsers(users []UserConfig) error {
	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.usersConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("store: writing %s: %w", usersConfigFile, err)
	}
	return nil
}

func (s *Store) AddUser(name, email string, isAdmin bool) (id, token string, err error) {
	if name == "" {
		return "", "", fmt.Errorf("store: user name is required")
	}

	users, err := s.readUsers()
	if err != nil {
		return "", "", err
	}
	if !isAdmin && len(users) == 0 {
		return "", "", fmt.Errorf("store: this project has no Maintainer yet — the first user added must be one")
	}

	token, err = generateToken()
	if err != nil {
		return "", "", err
	}
	idBytes := make([]byte, 6)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("store: generating user id: %w", err)
	}
	id = hex.EncodeToString(idBytes)

	users = append(users, UserConfig{ID: id, Name: name, Email: email, TokenHash: hashToken(token), IsAdmin: isAdmin})
	if err := s.writeUsers(users); err != nil {
		return "", "", err
	}
	return id, token, nil
}

func (s *Store) RegenerateToken(id string) (token string, err error) {
	users, err := s.readUsers()
	if err != nil {
		return "", err
	}
	idx := -1
	for i, u := range users {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("store: no user with id %q", id)
	}

	token, err = generateToken()
	if err != nil {
		return "", err
	}
	users[idx].TokenHash = hashToken(token)
	if err := s.writeUsers(users); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) RemoveUser(id string) error {
	users, err := s.readUsers()
	if err != nil {
		return err
	}
	out := make([]UserConfig, 0, len(users))
	removedWasAdmin := false
	remainingAdmins := 0
	for _, u := range users {
		if u.ID == id {
			removedWasAdmin = u.IsAdmin
			continue
		}
		out = append(out, u)
		if u.IsAdmin {
			remainingAdmins++
		}
	}
	if removedWasAdmin && remainingAdmins == 0 && len(out) > 0 {
		return fmt.Errorf("store: cannot remove the last Maintainer while other users still exist — promote someone else first, or remove every other user too")
	}
	return s.writeUsers(out)
}

func (s *Store) ListUsers() ([]UserConfig, error) {
	users, err := s.readUsers()
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Store) Authenticate(token string) (UserConfig, bool, error) {
	if token == "" {
		return UserConfig{}, false, nil
	}
	users, err := s.readUsers()
	if err != nil {
		return UserConfig{}, false, err
	}
	want := hashToken(token)
	for _, u := range users {
		if subtle.ConstantTimeCompare([]byte(u.TokenHash), []byte(want)) == 1 {
			return u, true, nil
		}
	}
	return UserConfig{}, false, nil
}
