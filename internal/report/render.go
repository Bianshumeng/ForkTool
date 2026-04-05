package report

import (
	"fmt"
	"os"
	"path/filepath"

	"forktool/pkg/model"
)

type Reporter interface {
	Format() string
	Render(report model.AuditReport) ([]byte, error)
}

func NewReporter(format string) (Reporter, error) {
	switch format {
	case "md":
		return markdownReporter{}, nil
	case "json":
		return jsonReporter{}, nil
	case "bd":
		return bdReporter{}, nil
	default:
		return nil, fmt.Errorf("unsupported report format %q", format)
	}
}

func WriteAll(report model.AuditReport, outputDir string, formats []string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create report directory: %w", err)
	}

	paths := make([]string, 0, len(formats))
	for _, format := range formats {
		reporter, err := NewReporter(format)
		if err != nil {
			return nil, err
		}

		content, err := reporter.Render(report)
		if err != nil {
			return nil, err
		}

		reportPath := filepath.Join(outputDir, "report."+reporter.Format())
		if err := os.WriteFile(reportPath, content, 0o644); err != nil {
			return nil, fmt.Errorf("write report %q: %w", reportPath, err)
		}
		paths = append(paths, filepath.ToSlash(reportPath))
	}

	return paths, nil
}

func PopulateSummary(report *model.AuditReport) {
	if report == nil {
		return
	}

	summary := model.AuditSummary{
		FeaturesScanned: len(report.Features),
	}

	for _, feature := range report.Features {
		for _, finding := range feature.Findings {
			switch finding.Severity {
			case "critical":
				summary.CriticalFindings++
			case "high":
				summary.HighFindings++
			case "medium":
				summary.MediumFindings++
			case "low":
				summary.LowFindings++
			}
		}
	}

	if summary.CriticalFindings > 0 || summary.HighFindings > 0 {
		summary.RecommendedAction = "Review high-risk findings before syncing official changes."
	} else if summary.FeaturesScanned > 0 {
		summary.RecommendedAction = "No high-risk drift detected by the currently supported deterministic rules."
	} else {
		summary.RecommendedAction = "No features were scanned."
	}

	report.Summary = summary
}

func HighestSeverity(report model.AuditReport) string {
	for _, feature := range report.Features {
		for _, finding := range feature.Findings {
			if finding.Severity == "critical" {
				return "critical"
			}
		}
	}
	for _, feature := range report.Features {
		for _, finding := range feature.Findings {
			if finding.Severity == "high" {
				return "high"
			}
		}
	}
	return ""
}
