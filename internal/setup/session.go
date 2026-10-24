package setup

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"elt/internal/csvio"
	"elt/internal/localize"
	"elt/internal/project"
	"elt/internal/store"
)

type Session struct {
	mu sync.Mutex

	dir       string
	dirStatus store.DirKind

	st       *store.Store
	finished bool

	warning string

	uploaded map[localize.FileType]uploadedFile
	skipped  map[localize.FileType]bool

	done chan *store.Store

	handedOff chan struct{}
}

type uploadedFile struct {
	Table  localize.Table
	Issues []csvio.Issue
}

func NewSession() *Session {
	return &Session{
		done:      make(chan *store.Store, 1),
		handedOff: make(chan struct{}),
		uploaded:  map[localize.FileType]uploadedFile{},
		skipped:   map[localize.FileType]bool{},
	}
}

func suggestedAuthorName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

func (s *Session) SetWarning(msg string) {
	s.warning = msg
}

func (s *Session) Done() <-chan *store.Store {
	return s.done
}

func (s *Session) MarkHandedOff() {
	close(s.handedOff)
}

func (s *Session) HandedOff() <-chan struct{} {
	return s.handedOff
}

func (s *Session) currentStep() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.step()
}

func (s *Session) step() string {
	switch {
	case s.finished:
		return "done"
	case s.st != nil:
		if s.nextUpload() != "" {
			return "upload"
		}
		return "languages"
	case s.dir != "":
		return "repository"
	default:
		return "directory"
	}
}

func (s *Session) nextUpload() localize.FileType {
	for _, ft := range project.StandardFileTypes {
		if _, ok := s.uploaded[ft]; ok {
			continue
		}
		if s.skipped[ft] {
			continue
		}
		return ft
	}
	return ""
}

func filterTargetLanguages(langs []string) []string {
	seen := make(map[string]bool, len(langs))
	out := make([]string, 0, len(langs))
	for _, lang := range langs {
		lang = strings.TrimSpace(lang)
		if lang == "" || lang == "English" || seen[lang] {
			continue
		}
		seen[lang] = true
		out = append(out, lang)
	}
	return out
}

func (s *Session) knownLanguagesFor() []string {
	if len(s.uploaded) == 0 {
		return knownLanguages()
	}
	seen := make(map[string]bool)
	out := make([]string, 0, 8)
	for _, ft := range project.StandardFileTypes {
		uf, ok := s.uploaded[ft]
		if !ok {
			continue
		}
		for _, lang := range filterTargetLanguages(uf.Table.Header.Languages) {
			if seen[lang] {
				continue
			}
			seen[lang] = true
			out = append(out, lang)
		}
	}
	return out
}

func (s *Session) chooseDirectory(dir string, create bool) (store.DirKind, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return "", fmt.Errorf("setup: already finished")
	}

	kind, err := store.ClassifyDir(dir)
	if err != nil {
		return "", fmt.Errorf("setup: checking %s: %w", dir, err)
	}
	if kind == store.DirOccupied {
		return "", fmt.Errorf("setup: %s already contains files that don't look like an elt project (a README and .git metadata alone are fine) — choose an empty directory or an existing project", dir)
	}
	if kind == store.DirEmpty && create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("setup: creating %s: %w", dir, err)
		}
	}

	s.dir = dir
	s.dirStatus = kind
	return kind, nil
}

func defaultProjectName(explicit, dir string) string {
	if explicit != "" {
		return explicit
	}
	base := filepath.Base(filepath.Clean(dir))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "New Project"
	}
	return base
}

type authorInput struct {
	Name  string
	Email string
}

func applyAuthor(st *store.Store, author authorInput) error {
	if author.Name == "" {
		return nil
	}
	email := author.Email
	if email == "" {
		email = emailSlug(author.Name) + "@elt.local"
	}
	return st.SetAuthor(store.AuthorConfig{Name: author.Name, Email: email})
}

func emailSlug(name string) string {
	var b strings.Builder
	prevDot := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDot = false
		case !prevDot:
			b.WriteByte('.')
			prevDot = true
		}
	}
	out := strings.TrimSuffix(b.String(), ".")
	if out == "" {
		return "user"
	}
	return out
}

func (s *Session) openExisting() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return fmt.Errorf("setup: already finished")
	}
	if s.dir == "" || s.dirStatus != store.DirProject {
		return fmt.Errorf("setup: no existing project found at the chosen directory")
	}

	st, err := store.Open(s.dir, "")
	if err != nil {
		return err
	}
	s.finish(st)
	return nil
}

func (s *Session) openLocal(projectName string, author authorInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return fmt.Errorf("setup: already finished")
	}
	if s.dir == "" || s.dirStatus != store.DirEmpty {
		return fmt.Errorf("setup: choose a new (empty) directory first")
	}

	st, err := store.Open(s.dir, defaultProjectName(projectName, s.dir))
	if err != nil {
		return err
	}
	if err := applyAuthor(st, author); err != nil {
		return err
	}
	s.st = st
	return nil
}

type remoteResult struct {
	JoinedExisting bool
}

func (s *Session) openRemote(projectName string, cfg store.RemoteConfig, author authorInput) (remoteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return remoteResult{}, fmt.Errorf("setup: already finished")
	}
	if s.dir == "" || s.dirStatus != store.DirEmpty {
		return remoteResult{}, fmt.Errorf("setup: choose a new (empty) directory first")
	}

	st, cloned, err := cloneOrInit(s.dir, projectName, cfg)
	if err != nil {
		return remoteResult{}, err
	}
	if err := applyAuthor(st, author); err != nil {
		return remoteResult{}, err
	}

	if cloned {
		s.finish(st)
		return remoteResult{JoinedExisting: true}, nil
	}
	s.st = st
	return remoteResult{JoinedExisting: false}, nil
}

func (s *Session) uploadFile(ft localize.FileType, table localize.Table, issues []csvio.Issue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return fmt.Errorf("setup: already finished")
	}
	if s.st == nil {
		return fmt.Errorf("setup: choose local or remote first")
	}
	s.uploaded[ft] = uploadedFile{Table: table, Issues: issues}
	delete(s.skipped, ft)
	return nil
}

func (s *Session) skipFile(ft localize.FileType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return fmt.Errorf("setup: already finished")
	}
	if s.st == nil {
		return fmt.Errorf("setup: choose local or remote first")
	}
	s.skipped[ft] = true
	delete(s.uploaded, ft)
	return nil
}

func (s *Session) finishLanguages(languages []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return fmt.Errorf("setup: already finished")
	}
	if s.st == nil {
		return fmt.Errorf("setup: no project opened yet")
	}
	if len(languages) == 0 {
		return fmt.Errorf("setup: at least one language is required")
	}
	if next := s.nextUpload(); next != "" {
		return fmt.Errorf("setup: upload or skip %s first", next)
	}

	p, err := s.st.Load()
	if err != nil {
		return err
	}
	p.TrackAll(languages)
	for _, ft := range project.StandardFileTypes {
		uf, ok := s.uploaded[ft]
		if !ok {
			continue
		}
		project.Sync(p.Files[ft], uf.Table)
	}

	msg := fmt.Sprintf("onboarding: track Localization/Dialogues/PDA (%s)", strings.Join(languages, ", "))
	if len(s.uploaded) > 0 {
		msg = fmt.Sprintf("onboarding: track + initial sync (%s)", strings.Join(languages, ", "))
	}
	if err := s.st.SaveAndCommit(p, msg); err != nil {
		return err
	}

	s.finish(s.st)
	return nil
}

func (s *Session) finish(st *store.Store) {
	s.finished = true
	s.st = st
	s.done <- st
}

func knownLanguages() []string {
	return localize.KnownLanguages
}
