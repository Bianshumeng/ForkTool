package report

import (
	"encoding/json"

	"forktool/pkg/model"
)

type jsonReporter struct{}

func (jsonReporter) Format() string {
	return "json"
}

func (jsonReporter) Render(report model.AuditReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
