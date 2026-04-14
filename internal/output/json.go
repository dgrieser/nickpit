package output

import (
	"encoding/json"
	"io"

	"github.com/dgrieser/nickpit/internal/model"
)

type JSONFormatter struct {
	w io.Writer
}

func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

func (f *JSONFormatter) FormatFindings(result *model.ReviewResult) error {
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
