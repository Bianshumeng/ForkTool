package report

import (
	"fmt"
	"strings"

	"forktool/pkg/model"
)

type bdReporter struct{}

func (bdReporter) Format() string {
	return "bd.md"
}

func (bdReporter) Render(report model.AuditReport) ([]byte, error) {
	var builder strings.Builder

	builder.WriteString("# ForkTool BD Draft\n\n")
	builder.WriteString("## Context\n\n")
	builder.WriteString(fmt.Sprintf("- Run ID: `%s`\n", report.RunID))
	builder.WriteString(fmt.Sprintf("- Local Repo: `%s`\n", report.LocalRepo.Path))
	builder.WriteString(fmt.Sprintf("- Official Repo: `%s`\n", report.OfficialRepo.Path))
	if report.ManifestPath != "" {
		builder.WriteString(fmt.Sprintf("- Manifest: `%s`\n", report.ManifestPath))
	}
	builder.WriteString("\n## Suggested Issues\n\n")

	index := 0
	for _, feature := range report.Features {
		if len(feature.Findings) == 0 {
			continue
		}
		index++
		builder.WriteString(fmt.Sprintf("### Draft %d: %s\n\n", index, feature.ID))
		builder.WriteString(fmt.Sprintf("- Suggested Title: `[%s] %s drift audit`\n", strings.ToUpper(feature.RiskLevel), feature.ID))
		builder.WriteString("- Suggested Type: `task`\n")
		builder.WriteString(fmt.Sprintf("- Suggested Labels: `%s`, `forktool-audit`\n", feature.ID))
		builder.WriteString("- Suggested Description:\n\n")
		builder.WriteString("```md\n")
		builder.WriteString("## Background\n")
		builder.WriteString(fmt.Sprintf("ForkTool detected semantic drift for `%s`.\n\n", feature.ID))
		builder.WriteString("## Goal\n")
		builder.WriteString("Align the local implementation with the official feature chain while preserving explicitly approved local hooks.\n\n")
		builder.WriteString("## Findings\n")
		for _, finding := range feature.Findings {
			builder.WriteString(fmt.Sprintf("- [%s] %s\n", strings.ToUpper(finding.Severity), finding.Title))
			if finding.Description != "" {
				builder.WriteString(fmt.Sprintf("  - %s\n", finding.Description))
			}
			for _, evidence := range finding.Evidence {
				if evidence.FilePath == "" {
					continue
				}
				builder.WriteString(fmt.Sprintf("  - %s: `%s%s%s`\n",
					evidence.RepoSide,
					evidence.FilePath,
					renderSymbolSuffix(evidence.SymbolName),
					renderLineSuffix(evidence.StartLine, evidence.EndLine),
				))
			}
			builder.WriteString(fmt.Sprintf("  - Recommendation: `%s`\n", finding.RecommendedAction))
			builder.WriteString(fmt.Sprintf("  - Decision Tag: `%s`\n", finding.DecisionTag))
		}
		builder.WriteString("\n## Acceptance Criteria\n")
		builder.WriteString("- Reported high-risk findings are resolved or explicitly accepted with documented local hooks.\n")
		builder.WriteString("- Relevant tests and feature-chain scans pass.\n")
		builder.WriteString("- Updated ForkTool report is attached for verification.\n")
		builder.WriteString("```\n\n")
	}

	if index == 0 {
		builder.WriteString("_No findings. No bd draft is needed._\n")
	}

	return []byte(builder.String()), nil
}
