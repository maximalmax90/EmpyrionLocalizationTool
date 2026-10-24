package project

import (
	"time"

	"elt/internal/localize"
)

type KeyStatus string

const (
	KeyActive  KeyStatus = "active"
	KeyRemoved KeyStatus = "removed"
)

type Translation struct {
	Text        string
	Source      string
	Stale       bool
	Issues      []string
	NeedsReview bool
}

func (t Translation) NeedsWork() bool {
	return t.Text == "" || t.Stale || len(t.Issues) > 0
}

type KeyRecord struct {
	English      string
	Status       KeyStatus
	Translations map[string]Translation
}

type BatchEntry struct {
	English string
	Draft   string
}

type Batch struct {
	ID        string
	Lang      string
	CreatedAt time.Time
	Keys      map[string]BatchEntry

	AssignedTo string
}

type FileTrack struct {
	FileType    localize.FileType
	Languages   []string
	Keys        map[string]*KeyRecord
	Order       []string
	OpenBatches map[string]*Batch
}

func NewFileTrack(ft localize.FileType, languages []string) *FileTrack {

	langs := make([]string, 0, len(languages))
	langs = append(langs, languages...)
	return &FileTrack{
		FileType:    ft,
		Languages:   langs,
		Keys:        map[string]*KeyRecord{},
		OpenBatches: map[string]*Batch{},
	}
}

type Project struct {
	Name     string
	Files    map[localize.FileType]*FileTrack
	Glossary map[string][]GlossaryTerm

	ChainSource map[localize.FileType][]byte
}

func NewProject(name string) *Project {
	return &Project{
		Name:        name,
		Files:       map[localize.FileType]*FileTrack{},
		Glossary:    map[string][]GlossaryTerm{},
		ChainSource: map[localize.FileType][]byte{},
	}
}

func (p *Project) TrackFile(ft localize.FileType, languages []string) *FileTrack {
	if existing, ok := p.Files[ft]; ok {
		return existing
	}
	track := NewFileTrack(ft, languages)
	p.Files[ft] = track
	return track
}

var StandardFileTypes = []localize.FileType{
	localize.FileTypeLocalization,
	localize.FileTypeDialogues,
	localize.FileTypePDA,
}

func (p *Project) TrackAll(languages []string) map[localize.FileType]*FileTrack {
	tracks := make(map[localize.FileType]*FileTrack, len(StandardFileTypes))
	for _, ft := range StandardFileTypes {
		tracks[ft] = p.TrackFile(ft, languages)
	}
	return tracks
}

func (p *Project) AddLanguage(lang string) {
	for _, track := range p.Files {
		found := false
		for _, existing := range track.Languages {
			if existing == lang {
				found = true
				break
			}
		}
		if !found {
			track.Languages = append(track.Languages, lang)
		}
	}
}

func (p *Project) RemoveLanguage(lang string) {
	for _, track := range p.Files {
		kept := make([]string, 0, len(track.Languages))
		for _, existing := range track.Languages {
			if existing != lang {
				kept = append(kept, existing)
			}
		}
		track.Languages = kept
	}
}
