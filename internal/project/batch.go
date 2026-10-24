package project

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func Split(ft *FileTrack, lang string, n int, onlyNeedsWork bool) ([]*Batch, error) {
	if n < 1 {
		return nil, fmt.Errorf("project: split count must be >= 1, got %d", n)
	}
	openForLang := 0
	for _, b := range ft.OpenBatches {
		if b.Lang == lang {
			openForLang++
		}
	}
	if openForLang > 0 {
		return nil, fmt.Errorf("project: %d batch(es) for %s are still open — merge or reclaim them all before splitting %s again", openForLang, lang, lang)
	}

	var candidates []string
	for _, key := range ft.Order {
		rec, ok := ft.Keys[key]
		if !ok || rec.Status != KeyActive {
			continue
		}
		if onlyNeedsWork && !rec.Translations[lang].NeedsWork() {
			continue
		}
		candidates = append(candidates, key)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	batches := make([]*Batch, n)
	for i := range batches {
		batches[i] = &Batch{ID: newBatchID(), Lang: lang, CreatedAt: time.Now().UTC(), Keys: map[string]BatchEntry{}}
	}
	for i, key := range candidates {
		b := batches[i%n]
		rec := ft.Keys[key]
		b.Keys[key] = BatchEntry{English: rec.English, Draft: rec.Translations[lang].Text}
	}

	var result []*Batch
	if ft.OpenBatches == nil {
		ft.OpenBatches = map[string]*Batch{}
	}
	for _, b := range batches {
		if len(b.Keys) == 0 {
			continue
		}
		ft.OpenBatches[b.ID] = b
		result = append(result, b)
	}
	return result, nil
}

type MergeResult struct {
	Applied []string
	Skipped []string
}

func MergeBatch(ft *FileTrack, batchID string, translations map[string]string) (MergeResult, error) {
	batch, ok := ft.OpenBatches[batchID]
	if !ok {
		return MergeResult{}, fmt.Errorf("project: batch %q is not open (already merged, reclaimed, or never existed)", batchID)
	}
	for key := range translations {
		if _, ok := batch.Keys[key]; !ok {
			return MergeResult{}, fmt.Errorf("project: batch %q does not contain key %q", batchID, key)
		}
	}

	res := MergeResult{Applied: make([]string, 0, len(translations)), Skipped: make([]string, 0)}
	for key, text := range translations {
		rec, ok := ft.Keys[key]
		if !ok || rec.Status != KeyActive {
			res.Skipped = append(res.Skipped, key)
			continue
		}
		entry := batch.Keys[key]
		tr := Translation{
			Text:   text,
			Source: "manual",
			Stale:  rec.English != entry.English,
		}
		rec.Translations[batch.Lang] = tr
		res.Applied = append(res.Applied, key)
	}

	delete(ft.OpenBatches, batchID)
	return res, nil
}

func AssignBatch(ft *FileTrack, batchID, assignedTo string) error {
	batch, ok := ft.OpenBatches[batchID]
	if !ok {
		return fmt.Errorf("project: batch %q is not open", batchID)
	}
	batch.AssignedTo = assignedTo
	return nil
}

func Reclaim(ft *FileTrack, batchID string) error {
	if _, ok := ft.OpenBatches[batchID]; !ok {
		return fmt.Errorf("project: batch %q is not open", batchID)
	}
	delete(ft.OpenBatches, batchID)
	return nil
}

func newBatchID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
