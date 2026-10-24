package store

import "fmt"

func (s *Store) Rollback(revision, message string) error {
	hash, err := s.Resolve(revision)
	if err != nil {
		return fmt.Errorf("store: rollback: %w", err)
	}
	p, err := s.LoadAt(hash)
	if err != nil {
		return fmt.Errorf("store: rollback: %w", err)
	}
	if message == "" {
		message = fmt.Sprintf("rollback to %s", hash)
	}
	return s.SaveAndCommit(p, message)
}
