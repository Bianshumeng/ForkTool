package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	reporting "forktool/internal/report"
	"forktool/pkg/model"
)

func (c *CLI) newReportCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "report",
		Short: "Report rendering commands",
	}

	var inputPath string
	var format string
	var outputPath string

	renderCommand := &cobra.Command{
		Use:   "render",
		Short: "Render a saved JSON audit report into Markdown or JSON output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(inputPath) == "" {
				return withExitCode(fmt.Errorf("input report path is required"), ExitInput)
			}

			absoluteInputPath, err := filepath.Abs(inputPath)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			content, err := os.ReadFile(absoluteInputPath)
			if err != nil {
				return withExitCode(fmt.Errorf("read input report: %w", err), ExitInput)
			}

			var auditReport model.AuditReport
			if err := json.Unmarshal(content, &auditReport); err != nil {
				return withExitCode(fmt.Errorf("parse input report: %w", err), ExitInput)
			}

			reporter, err := reporting.NewReporter(strings.ToLower(strings.TrimSpace(format)))
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			rendered, err := reporter.Render(auditReport)
			if err != nil {
				return err
			}

			if strings.TrimSpace(outputPath) == "" {
				_, writeErr := cmd.OutOrStdout().Write(rendered)
				return writeErr
			}

			absoluteOutputPath, err := filepath.Abs(outputPath)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			if err := os.MkdirAll(filepath.Dir(absoluteOutputPath), 0o755); err != nil {
				return withExitCode(fmt.Errorf("create report output directory: %w", err), ExitInput)
			}
			if err := os.WriteFile(absoluteOutputPath, rendered, 0o644); err != nil {
				return withExitCode(fmt.Errorf("write output report: %w", err), ExitInput)
			}
			return nil
		},
	}

	renderCommand.Flags().StringVar(&inputPath, "input", "", "input report JSON path")
	renderCommand.Flags().StringVar(&format, "format", "md", "output format: md or json")
	renderCommand.Flags().StringVar(&outputPath, "out", "", "optional output file path")
	command.AddCommand(renderCommand)
	return command
}
