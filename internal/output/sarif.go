package output

import (
	"fmt"

	"github.com/dgrieser/nickpit/internal/model"
)

type SARIFFormatter struct{}

func NewSARIFFormatter() *SARIFFormatter {
	return &SARIFFormatter{}
}

func (f *SARIFFormatter) FormatFindings(_ *model.ReviewResult) error {
	return fmt.Errorf("SARIF output not yet implemented")
}
