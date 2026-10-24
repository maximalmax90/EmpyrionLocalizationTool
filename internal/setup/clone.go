package setup

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"elt/internal/store"
)

func cloneOrInit(dir, projectName string, cfg store.RemoteConfig) (st *store.Store, cloned bool, err error) {
	st, err = store.CloneRaw(dir, cfg)
	if err != nil {
		if !errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil, false, err
		}
		return initFreshInto(dir, projectName, cfg)
	}

	kind, err := store.ClassifyDir(dir)
	if err != nil {
		return nil, false, err
	}
	switch kind {
	case store.DirProject:
		return st, true, nil
	case store.DirEmpty:

		return initFreshInto(dir, projectName, cfg)
	default:
		_ = os.RemoveAll(dir)
		return nil, false, fmt.Errorf("setup: %s already has content that isn't an elt project — choose a different remote", cfg.URL)
	}
}

func initFreshInto(dir, projectName string, cfg store.RemoteConfig) (*store.Store, bool, error) {
	st, err := store.Open(dir, defaultProjectName(projectName, dir))
	if err != nil {
		return nil, false, err
	}
	if err := st.SetRemote(cfg); err != nil {
		return nil, false, err
	}
	return st, false, nil
}
