package csvio

import (
	"encoding/csv"
	"io"

	"elt/internal/localize"
)

func Compile(w io.Writer, table localize.Table) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := make([]string, 0, 1+len(table.Header.Languages))
	header = append(header, table.Header.KeyColumn)
	header = append(header, table.Header.Languages...)
	if err := cw.Write(header); err != nil {
		return err
	}

	for _, row := range table.Rows {
		fields := make([]string, 0, len(header))
		fields = append(fields, row.Key)
		for _, lang := range table.Header.Languages {
			fields = append(fields, row.Value(lang))
		}
		if err := cw.Write(fields); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}
