package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"elt/internal/content"
	"elt/internal/localize"
	"elt/internal/project"
)

type chainNodeProgress struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

type chainTreeNodeResponse struct {
	Label       string                   `json:"label"`
	Key         string                   `json:"key,omitempty"`
	Kind        string                   `json:"kind"`
	English     string                   `json:"english,omitempty"`
	Translation string                   `json:"translation,omitempty"`
	Progress    chainNodeProgress        `json:"progress"`
	Children    []*chainTreeNodeResponse `json:"children,omitempty"`
}

type chainTreeResponse struct {
	Root   *chainTreeNodeResponse `json:"root"`
	Capped bool                   `json:"capped"`
}

type UploadChainSourceResponse struct {
	Roots int `json:"roots"`
}

func (s *Server) handleUploadChainSource(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	if !ft.SupportsChains() {
		writeError(w, http.StatusBadRequest, "%s has no chain graph (only Dialogues and PDA do)", ft)
		return
	}

	maxBody := s.maxSyncBodyBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds %d bytes", maxBody)
			return
		}
		writeError(w, http.StatusBadRequest, "reading request body: %v", err)
		return
	}
	roots, err := chainRootsFrom(ft, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	project.SetChainSource(s.proj, ft, body)
	msg := fmt.Sprintf("upload %s chain source (%d roots)", ft, len(roots))
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, UploadChainSourceResponse{Roots: len(roots)})
}

func (s *Server) handleChainRoots(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	if !ft.SupportsChains() {
		writeError(w, http.StatusBadRequest, "%s has no chain graph (only Dialogues and PDA do)", ft)
		return
	}

	source, ok := s.chainSource(ft)
	if !ok {
		writeError(w, http.StatusConflict, "no chain source uploaded for %s yet — an admin must POST /files/%s/chains/source first", ft, ft)
		return
	}
	roots, err := chainRootsFrom(ft, source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, roots)
}

func (s *Server) handleChain(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	if !ft.SupportsChains() {
		writeError(w, http.StatusBadRequest, "%s has no chain graph (only Dialogues and PDA do)", ft)
		return
	}
	root := r.PathValue("root")

	source, ok := s.chainSource(ft)
	if !ok {
		writeError(w, http.StatusConflict, "no chain source uploaded for %s yet — an admin must POST /files/%s/chains/source first", ft, ft)
		return
	}
	keys, err := chainFrom(ft, source, root)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleChainTree(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	if !ft.SupportsChains() {
		writeError(w, http.StatusBadRequest, "%s has no chain graph (only Dialogues and PDA do)", ft)
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	root := r.PathValue("root")

	source, ok := s.chainSource(ft)
	if !ok {
		writeError(w, http.StatusConflict, "no chain source uploaded for %s yet — an admin must POST /files/%s/chains/source first", ft, ft)
		return
	}
	tree, capped, err := chainTreeFrom(ft, source, root)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	s.mu.Lock()
	track, _ := s.track(ft)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, chainTreeResponse{Root: annotateChainTree(tree, track, lang), Capped: capped})
}

func annotateChainTree(n *content.ChainTreeNode, track *project.FileTrack, lang string) *chainTreeNodeResponse {
	out := &chainTreeNodeResponse{Label: n.Label, Key: n.Key, Kind: n.Kind}
	if n.Key != "" {
		out.Progress.Total = 1
		if track != nil {
			if rec, ok := track.Keys[n.Key]; ok {
				out.English = rec.English
				if tr, ok := rec.Translations[lang]; ok {
					out.Translation = tr.Text
				}
			}
		}
		if chainKeyIsDone(track, n.Key, lang) {
			out.Progress.Done = 1
		}
	}
	for _, c := range n.Children {
		child := annotateChainTree(c, track, lang)
		out.Children = append(out.Children, child)
		out.Progress.Total += child.Progress.Total
		out.Progress.Done += child.Progress.Done
	}
	return out
}

func chainKeyIsDone(track *project.FileTrack, key, lang string) bool {
	if track == nil {
		return false
	}
	rec, ok := track.Keys[key]
	if !ok {
		return false
	}
	tr, ok := rec.Translations[lang]
	if !ok {
		return false
	}
	return !tr.NeedsWork() && tr.Source != "machine"
}

func chainTreeFrom(ft localize.FileType, source []byte, root string) (*content.ChainTreeNode, bool, error) {
	switch ft {
	case localize.FileTypeDialogues:
		graph, err := content.ParseDialoguesECF(bytes.NewReader(source))
		if err != nil {
			return nil, false, fmt.Errorf("parsing Dialogues.ecf: %w", err)
		}
		if _, ok := graph[root]; !ok {
			return nil, false, fmt.Errorf("no dialogue named %q", root)
		}
		tree, capped := content.DialogueChainTree(graph, root, 0)
		return tree, capped, nil
	case localize.FileTypePDA:
		chapters, err := content.ParsePDAYAML(bytes.NewReader(source))
		if err != nil {
			return nil, false, fmt.Errorf("parsing PDA.yaml: %w", err)
		}
		found := false
		for _, r := range content.PDAChainRoots(chapters) {
			if r == root {
				found = true
				break
			}
		}
		if !found {
			return nil, false, fmt.Errorf("no chapter titled %q", root)
		}
		tree, capped := content.PDAChainTree(chapters, root, 0)
		return tree, capped, nil
	default:
		return nil, false, fmt.Errorf("%s has no chain graph", ft)
	}
}

func (s *Server) chainSource(ft localize.FileType) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.proj.ChainSource[ft]
	return data, ok
}

func chainRootsFrom(ft localize.FileType, source []byte) ([]string, error) {
	switch ft {
	case localize.FileTypeDialogues:
		graph, err := content.ParseDialoguesECF(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("parsing Dialogues.ecf: %w", err)
		}
		return content.DialogueChainRoots(graph), nil
	case localize.FileTypePDA:
		chapters, err := content.ParsePDAYAML(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("parsing PDA.yaml: %w", err)
		}
		return content.PDAChainRoots(chapters), nil
	default:
		return nil, fmt.Errorf("%s has no chain graph", ft)
	}
}

func chainFrom(ft localize.FileType, source []byte, root string) ([]string, error) {
	switch ft {
	case localize.FileTypeDialogues:
		graph, err := content.ParseDialoguesECF(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("parsing Dialogues.ecf: %w", err)
		}
		if _, ok := graph[root]; !ok {
			return nil, fmt.Errorf("no dialogue named %q", root)
		}
		return content.DialogueChain(graph, root), nil
	case localize.FileTypePDA:
		chapters, err := content.ParsePDAYAML(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("parsing PDA.yaml: %w", err)
		}
		found := false
		for _, r := range content.PDAChainRoots(chapters) {
			if r == root {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("no chapter titled %q", root)
		}
		return content.PDAChain(chapters, root), nil
	default:
		return nil, fmt.Errorf("%s has no chain graph", ft)
	}
}
