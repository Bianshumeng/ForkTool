package report

import (
	"fmt"
	"strings"

	"forktool/pkg/model"
)

type markdownReporter struct{}

func (markdownReporter) Format() string {
	return "md"
}

func (markdownReporter) Render(report model.AuditReport) ([]byte, error) {
	var builder strings.Builder

	builder.WriteString("# ForkTool Audit Report\n\n")
	builder.WriteString("## Context\n\n")
	builder.WriteString(fmt.Sprintf("- Local Repo: `%s`\n", report.LocalRepo.Path))
	builder.WriteString(fmt.Sprintf("- Official Repo: `%s`\n", report.OfficialRepo.Path))
	if report.OfficialRepo.Tag != "" {
		builder.WriteString(fmt.Sprintf("- Official Tag: `%s`\n", report.OfficialRepo.Tag))
	}
	if report.OfficialRepo.Commit != "" {
		builder.WriteString(fmt.Sprintf("- Official Commit: `%s`\n", report.OfficialRepo.Commit))
	}
	if report.ManifestPath != "" {
		builder.WriteString(fmt.Sprintf("- Manifest: `%s`\n", report.ManifestPath))
	}
	if report.DecisionFilePath != "" {
		builder.WriteString(fmt.Sprintf("- Decision File: `%s`\n", report.DecisionFilePath))
	}
	if report.Baseline != nil {
		builder.WriteString(fmt.Sprintf("- Baseline Verified: `%t`\n", report.Baseline.Valid))
	}
	builder.WriteString("\n## Summary\n\n")
	builder.WriteString(fmt.Sprintf("- Features Scanned: `%d`\n", report.Summary.FeaturesScanned))
	builder.WriteString(fmt.Sprintf("- Critical Findings: `%d`\n", report.Summary.CriticalFindings))
	builder.WriteString(fmt.Sprintf("- High Findings: `%d`\n", report.Summary.HighFindings))
	builder.WriteString(fmt.Sprintf("- Medium Findings: `%d`\n", report.Summary.MediumFindings))
	builder.WriteString(fmt.Sprintf("- Low Findings: `%d`\n", report.Summary.LowFindings))
	if report.Summary.RecommendedAction != "" {
		builder.WriteString(fmt.Sprintf("- Recommended Action: `%s`\n", report.Summary.RecommendedAction))
	}

	for _, feature := range report.Features {
		builder.WriteString(fmt.Sprintf("\n## Feature: %s\n\n", feature.ID))
		if feature.Name != "" {
			builder.WriteString(fmt.Sprintf("- Name: `%s`\n", feature.Name))
		}
		builder.WriteString(fmt.Sprintf("- Risk Level: `%s`\n", feature.RiskLevel))
		builder.WriteString(fmt.Sprintf("- Status: `%s`\n", feature.Status))
		builder.WriteString(fmt.Sprintf("- Local Nodes: `%d`\n", len(feature.LocalNodes)))
		builder.WriteString(fmt.Sprintf("- Official Nodes: `%d`\n", len(feature.OfficialNodes)))

		if len(feature.Notes) > 0 {
			builder.WriteString("- Notes:\n")
			for _, note := range feature.Notes {
				builder.WriteString(fmt.Sprintf("  - %s\n", note))
			}
		}

		if len(feature.Findings) == 0 {
			builder.WriteString("\n_No findings yet. This report currently reflects discovery and report plumbing only._\n")
			continue
		}

		for index, finding := range feature.Findings {
			builder.WriteString(fmt.Sprintf("\n### Finding %d\n\n", index+1))
			builder.WriteString(fmt.Sprintf("- Severity: `%s`\n", finding.Severity))
			builder.WriteString(fmt.Sprintf("- Category: `%s`\n", finding.Category))
			builder.WriteString(fmt.Sprintf("- Decision: `%s`\n", finding.DecisionTag))
			builder.WriteString(fmt.Sprintf("- Title: `%s`\n", finding.Title))
			if finding.Description != "" {
				builder.WriteString(fmt.Sprintf("- Description: `%s`\n", finding.Description))
			}
			if len(finding.Evidence) > 0 {
				builder.WriteString("- Evidence:\n")
				for _, evidence := range finding.Evidence {
					if evidence.FilePath != "" {
						builder.WriteString(fmt.Sprintf("  - %s: `%s%s%s`\n",
							evidence.RepoSide,
							evidence.FilePath,
							renderSymbolSuffix(evidence.SymbolName),
							renderLineSuffix(evidence.StartLine, evidence.EndLine),
						))
					}
				}
			}
			builder.WriteString(fmt.Sprintf("- Recommendation: `%s`\n", finding.RecommendedAction))
		}
	}

	actionPlan := buildActionPlan(report)
	if len(actionPlan) > 0 {
		builder.WriteString("\n## Action Plan\n\n")
		for index, action := range actionPlan {
			builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, action))
		}
	}

	return []byte(builder.String()), nil
}

func renderSymbolSuffix(symbol string) string {
	if strings.TrimSpace(symbol) == "" {
		return ""
	}
	return "#" + symbol
}

func renderLineSuffix(startLine, endLine int) string {
	if startLine <= 0 {
		return ""
	}
	if endLine > 0 && endLine != startLine {
		return fmt.Sprintf(":%d-%d", startLine, endLine)
	}
	return fmt.Sprintf(":%d", startLine)
}

func buildActionPlan(report model.AuditReport) []string {
	actions := make([]string, 0)
	seen := make(map[string]struct{})
	for _, feature := range report.Features {
		for _, finding := range feature.Findings {
			action := strings.TrimSpace(finding.RecommendedAction)
			if action == "" {
				continue
			}
			line := fmt.Sprintf("%s: %s", feature.ID, action)
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			actions = append(actions, line)
		}
	}
	return actions
}
