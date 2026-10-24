package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"elt/internal/project"
)

type CommitInfo struct {
	Hash    string
	When    time.Time
	Message string
}

var errStopIteration = errors.New("store: stop iteration")

func (s *Store) History(limit int) ([]CommitInfo, error) {
	head, err := s.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("store: reading HEAD: %w", err)
	}

	iter, err := s.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("store: reading history: %w", err)
	}
	defer iter.Close()

	var out []CommitInfo
	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && len(out) >= limit {
			return errStopIteration
		}
		out = append(out, CommitInfo{
			Hash:    c.Hash.String(),
			When:    c.Author.When,
			Message: c.Message,
		})
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return nil, fmt.Errorf("store: walking history: %w", err)
	}
	return out, nil
}

func (s *Store) LoadAt(revision string) (*project.Project, error) {
	hash, err := s.repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("store: resolving %q: %w", revision, err)
	}
	commit, err := s.repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("store: reading commit %s: %w", hash, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("store: reading tree at %s: %w", hash, err)
	}

	readFile := func(name string) ([]byte, error) {
		f, err := tree.File(name)
		if err != nil {
			return nil, err
		}
		r, err := f.Reader()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}

	manifestData, err := readFile(manifestFileName)
	if err != nil {
		return nil, fmt.Errorf("store: revision %s has no %s: %w", revision, manifestFileName, err)
	}
	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		return nil, fmt.Errorf("store: %s at revision %s is not valid: %w", manifestFileName, revision, err)
	}
	if err := validateManifestVersion(m); err != nil {
		return nil, err
	}

	return assembleProject(m, readFile)
}

func (s *Store) Resolve(revision string) (string, error) {
	hash, err := s.repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return "", fmt.Errorf("store: resolving %q: %w", revision, err)
	}
	return hash.String(), nil
}
