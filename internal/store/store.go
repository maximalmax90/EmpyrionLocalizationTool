package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"elt/internal/localize"
	"elt/internal/project"
)

const manifestFileName = "project.json"

const glossaryFileName = "glossary.json"

var chainSourceFileNames = map[localize.FileType]string{
	localize.FileTypeDialogues: "Dialogues.ecf",
	localize.FileTypePDA:       "PDA.yaml",
}

const currentManifestVersion = 1

type manifest struct {
	Version int                 `json:"version"`
	Name    string              `json:"name"`
	Files   []localize.FileType `json:"files"`
}

func validateManifestVersion(m manifest) error {
	if m.Version > currentManifestVersion {
		return fmt.Errorf("store: %s has schema version %d, but this build of elt only understands up to version %d — upgrade elt to open this project", manifestFileName, m.Version, currentManifestVersion)
	}
	return nil
}

type Store struct {
	dir  string
	repo *git.Repository
}

func (s *Store) Dir() string { return s.dir }

type DirKind string

const (
	DirEmpty DirKind = "empty"

	DirProject DirKind = "project"

	DirOccupied DirKind = "occupied"
)

func isScaffoldEntry(name string) bool {
	if strings.HasPrefix(name, ".git") {
		return true
	}
	if name == localConfigDirName {

		return true
	}
	switch strings.ToLower(name) {
	case "readme.md", "readme", "readme.txt":
		return true
	}
	return false
}

type dirScan struct {
	hasManifest bool
	hasGit      bool
	hasOther    bool
}

func scanDir(dir string) (dirScan, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return dirScan{}, nil
	}
	if err != nil {
		return dirScan{}, err
	}
	var s dirScan
	for _, e := range entries {
		switch name := e.Name(); {
		case name == manifestFileName:
			s.hasManifest = true
		case name == ".git":
			s.hasGit = true
		case isScaffoldEntry(name):

		default:
			s.hasOther = true
		}
	}
	return s, nil
}

func (s dirScan) kind() DirKind {
	switch {
	case s.hasManifest:
		return DirProject
	case s.hasOther:
		return DirOccupied
	default:
		return DirEmpty
	}
}

func ClassifyDir(dir string) (DirKind, error) {
	scan, err := scanDir(dir)
	if err != nil {
		return "", err
	}
	return scan.kind(), nil
}

func Open(dir, projectName string) (*Store, error) {
	scan, err := scanDir(dir)
	if err != nil {
		return nil, fmt.Errorf("store: checking %s: %w", dir, err)
	}
	switch scan.kind() {
	case DirProject:
		return openExisting(dir)
	case DirOccupied:
		return nil, fmt.Errorf("store: %s already contains files that don't look like an elt project (a README and .git metadata alone are fine) — point at an empty directory or an existing elt project", dir)
	default:
		return initStore(dir, projectName, scan.hasGit)
	}
}

func initStore(dir, projectName string, hasGit bool) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: creating %s: %w", dir, err)
	}

	var repo *git.Repository
	var err error
	if hasGit {

		repo, err = git.PlainOpen(dir)
		if err != nil {
			return nil, fmt.Errorf("store: opening existing git repo in %s: %w", dir, err)
		}
	} else {
		repo, err = git.PlainInit(dir, false)
		if err != nil {
			return nil, fmt.Errorf("store: git init in %s: %w", dir, err)
		}
	}

	s := &Store{dir: dir, repo: repo}
	if err := s.ensureGitignoreHasLocalConfigEntry(); err != nil {
		return nil, err
	}
	if err := s.writeManifest(manifest{Name: projectName}); err != nil {
		return nil, err
	}
	if err := s.Commit("initial empty project"); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureGitignoreHasLocalConfigEntry() error {
	entry := localConfigDirName + "/"
	path := filepath.Join(s.dir, ".gitignore")

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store: reading .gitignore: %w", err)
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("store: writing .gitignore: %w", err)
	}
	return nil
}

func openExisting(dir string) (*Store, error) {
	if _, err := os.Stat(filepath.Join(dir, manifestFileName)); err != nil {
		return nil, fmt.Errorf("store: %s does not look like a project store (no %s found): %w", dir, manifestFileName, err)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("store: %s has %s but is not a git repository: %w", dir, manifestFileName, err)
	}
	return &Store{dir: dir, repo: repo}, nil
}

func LooksLikeProjectDir(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, manifestFileName))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) manifestPath() string {
	return filepath.Join(s.dir, manifestFileName)
}

func (s *Store) fileTrackPath(ft localize.FileType) string {
	return filepath.Join(s.dir, string(ft)+".json")
}

func (s *Store) glossaryPath() string {
	return filepath.Join(s.dir, glossaryFileName)
}

func (s *Store) readManifest() (manifest, error) {
	data, err := os.ReadFile(s.manifestPath())
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("store: %s is not valid JSON: %w", manifestFileName, err)
	}
	if err := validateManifestVersion(m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func (s *Store) writeManifest(m manifest) error {
	m.Version = currentManifestVersion
	return writeJSONFile(s.manifestPath(), m)
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encoding %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("store: writing %s: %w", path, err)
	}
	return nil
}

func (s *Store) Load() (*project.Project, error) {
	m, err := s.readManifest()
	if err != nil {
		return nil, err
	}
	return assembleProject(m, func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(s.dir, name))
	})
}

func assembleProject(m manifest, readFile func(name string) ([]byte, error)) (*project.Project, error) {
	p := project.NewProject(m.Name)
	for _, ft := range m.Files {
		name := string(ft) + ".json"
		data, err := readFile(name)
		if err != nil {
			return nil, fmt.Errorf("store: reading %s: %w", name, err)
		}
		track := &project.FileTrack{}
		if err := json.Unmarshal(data, track); err != nil {
			return nil, fmt.Errorf("store: %s is not valid: %w", name, err)
		}
		p.Files[ft] = track
	}

	if data, err := readFile(glossaryFileName); err == nil {
		var glossary map[string][]project.GlossaryTerm
		if err := json.Unmarshal(data, &glossary); err != nil {
			return nil, fmt.Errorf("store: %s is not valid: %w", glossaryFileName, err)
		}
		p.Glossary = glossary
	} else if !isNotFoundError(err) {
		return nil, fmt.Errorf("store: reading %s: %w", glossaryFileName, err)
	}

	for ft, name := range chainSourceFileNames {
		data, err := readFile(name)
		if err == nil {
			if p.ChainSource == nil {
				p.ChainSource = map[localize.FileType][]byte{}
			}
			p.ChainSource[ft] = data
		} else if !isNotFoundError(err) {
			return nil, fmt.Errorf("store: reading %s: %w", name, err)
		}
	}

	return p, nil
}

func isNotFoundError(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, object.ErrFileNotFound)
}

func (s *Store) Save(p *project.Project) error {
	fileTypes := make([]localize.FileType, 0, len(p.Files))
	for ft := range p.Files {
		fileTypes = append(fileTypes, ft)
	}
	sort.Slice(fileTypes, func(i, j int) bool { return fileTypes[i] < fileTypes[j] })

	if err := s.writeManifest(manifest{Name: p.Name, Files: fileTypes}); err != nil {
		return err
	}
	for _, ft := range fileTypes {
		if err := writeJSONFile(s.fileTrackPath(ft), p.Files[ft]); err != nil {
			return err
		}
	}
	if err := writeJSONFile(s.glossaryPath(), p.Glossary); err != nil {
		return err
	}
	for ft, name := range chainSourceFileNames {
		data, ok := p.ChainSource[ft]
		if !ok {
			continue
		}
		if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
			return fmt.Errorf("store: writing %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) SaveAndCommit(p *project.Project, message string) error {
	if err := s.Save(p); err != nil {
		return err
	}
	return s.Commit(message)
}

func (s *Store) SaveAndCommitAs(p *project.Project, message string, author AuthorConfig) error {
	if err := s.Save(p); err != nil {
		return err
	}
	return s.CommitAs(message, author)
}

func (s *Store) Commit(message string) error {
	return s.commit(message, nil)
}

func (s *Store) CommitAs(message string, author AuthorConfig) error {
	return s.commit(message, &author)
}

func (s *Store) commit(message string, override *AuthorConfig) error {
	wt, err := s.repo.Worktree()
	if err != nil {
		return fmt.Errorf("store: opening worktree: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("store: listing %s: %w", s.dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, err := wt.Add(e.Name()); err != nil {
			return fmt.Errorf("store: staging %s: %w", e.Name(), err)
		}
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("store: checking status: %w", err)
	}
	if status.IsClean() {
		return nil
	}

	sig := s.commitSignature(override)
	_, err = wt.Commit(message, &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		return fmt.Errorf("store: committing: %w", err)
	}

	if cfg, ok, err := s.Remote(); err == nil && ok && cfg.Push == "on-commit" {
		if err := s.pushTo(cfg); err != nil {

			return fmt.Errorf("store: committed locally, but auto-push failed: %w", err)
		}
	}
	return nil
}
