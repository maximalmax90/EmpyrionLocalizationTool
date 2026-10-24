package store

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
)

func (s *Store) GC() error {
	if err := s.repo.Prune(git.PruneOptions{Handler: s.repo.DeleteObject}); err != nil && !errors.Is(err, git.ErrLooseObjectsNotSupported) {
		return fmt.Errorf("store: pruning: %w", err)
	}
	if err := s.repo.RepackObjects(&git.RepackConfig{}); err != nil && !errors.Is(err, git.ErrPackedObjectsNotSupported) {
		return fmt.Errorf("store: repacking: %w", err)
	}
	return nil
}
