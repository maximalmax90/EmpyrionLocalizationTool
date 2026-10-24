package project

import "elt/internal/localize"

func SetChainSource(p *Project, ft localize.FileType, data []byte) {
	if p.ChainSource == nil {
		p.ChainSource = map[localize.FileType][]byte{}
	}
	p.ChainSource[ft] = data
}

func HasChainSource(p *Project, ft localize.FileType) bool {
	_, ok := p.ChainSource[ft]
	return ok
}

func ClearChainSource(p *Project, ft localize.FileType) {
	delete(p.ChainSource, ft)
}
