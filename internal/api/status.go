package api

import (
	"net/http"

	"elt/internal/project"
)

type FileStatus struct {
	Languages   []string `json:"languages"`
	ActiveKeys  int      `json:"active_keys"`
	RemovedKeys int      `json:"removed_keys"`
	OpenBatches int      `json:"open_batches"`

	SupportsChains bool `json:"supports_chains"`

	HasChainSource bool `json:"has_chain_source"`
}

type StatusResponse struct {
	Name  string                `json:"name"`
	Files map[string]FileStatus `json:"files"`

	RepoSizeBytes int64 `json:"repo_size_bytes"`

	LoopbackOnly bool `json:"loopback_only"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	size, err := s.store.Size()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "measuring repository size: %v", err)
		return
	}

	resp := StatusResponse{Name: s.proj.Name, Files: map[string]FileStatus{}, RepoSizeBytes: size, LoopbackOnly: s.loopbackOnly}
	for ft, track := range s.proj.Files {
		var active, removed int
		for _, rec := range track.Keys {
			if rec.Status == project.KeyRemoved {
				removed++
			} else {
				active++
			}
		}
		languages := track.Languages
		if languages == nil {

			languages = []string{}
		}
		_, hasChainSource := s.proj.ChainSource[ft]
		resp.Files[string(ft)] = FileStatus{
			Languages:      languages,
			ActiveKeys:     active,
			RemovedKeys:    removed,
			OpenBatches:    len(track.OpenBatches),
			SupportsChains: ft.SupportsChains(),
			HasChainSource: hasChainSource,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
