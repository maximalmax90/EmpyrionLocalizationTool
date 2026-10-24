package store

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (s *Store) Compact(boundary, message string) (string, error) {
	boundaryHash, err := s.repo.ResolveRevision(plumbing.Revision(boundary))
	if err != nil {
		return "", fmt.Errorf("store: compact: resolving %q: %w", boundary, err)
	}

	head, err := s.repo.Head()
	if err != nil {
		return "", fmt.Errorf("store: compact: reading HEAD: %w", err)
	}

	iter, err := s.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return "", fmt.Errorf("store: compact: reading history: %w", err)
	}
	defer iter.Close()

	var afterBoundary []*object.Commit
	var boundaryCommit *object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == *boundaryHash {
			boundaryCommit = c
			return errStopIteration
		}
		afterBoundary = append(afterBoundary, c)
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return "", fmt.Errorf("store: compact: walking history: %w", err)
	}
	if boundaryCommit == nil {
		return "", fmt.Errorf("store: compact: %q is not an ancestor of the current history", boundary)
	}

	if message == "" {
		message = fmt.Sprintf("compacted history through %s", boundaryHash.String())
	}
	sig := *s.commitSignature(nil)
	newRoot := &object.Commit{
		Author:    sig,
		Committer: sig,
		Message:   message,
		TreeHash:  boundaryCommit.TreeHash,
	}
	tip, err := s.storeCommitObject(newRoot)
	if err != nil {
		return "", fmt.Errorf("store: compact: writing baseline commit: %w", err)
	}

	for i := len(afterBoundary) - 1; i >= 0; i-- {
		c := afterBoundary[i]
		replayed := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Message:      c.Message,
			TreeHash:     c.TreeHash,
			ParentHashes: []plumbing.Hash{tip},
		}
		tip, err = s.storeCommitObject(replayed)
		if err != nil {
			return "", fmt.Errorf("store: compact: replaying %s: %w", c.Hash, err)
		}
	}

	if err := s.updateHeadTo(tip); err != nil {
		return "", fmt.Errorf("store: compact: updating history: %w", err)
	}
	return tip.String(), nil
}

func (s *Store) storeCommitObject(c *object.Commit) (plumbing.Hash, error) {
	obj := s.repo.Storer.NewEncodedObject()
	if err := c.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.repo.Storer.SetEncodedObject(obj)
}

func (s *Store) updateHeadTo(hash plumbing.Hash) error {
	head, err := s.repo.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	name := plumbing.HEAD
	if head.Type() != plumbing.HashReference {
		name = head.Target()
	}
	return s.repo.Storer.SetReference(plumbing.NewHashReference(name, hash))
}
