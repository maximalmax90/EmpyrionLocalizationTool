package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) Size() (int64, error) {
	var total int64
	gitDir := filepath.Join(s.dir, ".git")
	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("store: measuring repository size: %w", err)
	}
	return total, nil
}
