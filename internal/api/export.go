package api

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var repoArchiveNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (s *Server) handleExportRepository(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "zip" && format != "tar.gz" {
		writeError(w, http.StatusBadRequest, `?format= must be "zip" or "tar.gz"`)
		return
	}

	s.mu.Lock()
	name := s.proj.Name
	dir := s.store.Dir()
	s.mu.Unlock()

	base := repoArchiveNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	if base == "" {
		base = "project"
	}

	if format == "zip" {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, base))
		_ = writeZipArchive(w, dir, base)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, base))
	_ = writeTarGzArchive(w, dir, base)
}

func writeZipArchive(w io.Writer, dir, base string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()
	return walkArchivable(dir, func(relPath string, info os.FileInfo, content io.Reader) error {
		fw, err := zw.Create(filepath.ToSlash(filepath.Join(base, relPath)))
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, content)
		return err
	})
}

func writeTarGzArchive(w io.Writer, dir, base string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	return walkArchivable(dir, func(relPath string, info os.FileInfo, content io.Reader) error {
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(filepath.Join(base, relPath))
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tw, content)
		return err
	})
}

func walkArchivable(dir string, add func(relPath string, info os.FileInfo, content io.Reader) error) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".elt-local" {
				return filepath.SkipDir
			}
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return add(filepath.ToSlash(rel), info, f)
	})
}
