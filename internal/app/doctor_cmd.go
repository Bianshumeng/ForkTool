package app

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"forktool/internal/manifest"
	"forktool/pkg/cliui"
	"forktool/pkg/model"
)

type doctorResult struct {
	ToolVersion string                `json:"toolVersion"`
	GoVersion   string                `json:"goVersion"`
	Workdir     string                `json:"workdir"`
	ConfigPath  string                `json:"configPath"`
	ConfigFound bool                  `json:"configFound"`
	Ready       bool                  `json:"ready"`
	Checks      []model.BaselineCheck `json:"checks"`
}

func (c *CLI) newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Inspect workspace readiness for ForkTool MVP commands",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			configPath := resolveConfigPath(workdir, c.configPath)
			workspace, found, err := loadWorkspaceIfExists(configPath)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			result := doctorResult{
				ToolVersion: Version,
				GoVersion:   runtime.Version(),
				Workdir:     filepath.ToSlash(workdir),
				ConfigPath:  filepath.ToSlash(configPath),
				ConfigFound: found,
				Ready:       true,
			}

			workspaceDir := filepath.Join(workdir, ".forktool")
			result.Checks = append(result.Checks, checkDirectory("workspace-dir", workspaceDir))
			result.Checks = append(result.Checks, checkDirectory("runs-dir", filepath.Join(workspaceDir, "runs")))
			result.Checks = append(result.Checks, checkDirectory("cache-dir", filepath.Join(workspaceDir, "cache")))

			if found {
				manifestPath := resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Manifest.Path)
				if _, statErr := os.Stat(manifestPath); statErr != nil {
					result.Ready = false
					result.Checks = append(result.Checks, model.BaselineCheck{
						Name:   "manifest-file",
						Passed: false,
						Detail: statErr.Error(),
					})
				} else {
					validation := manifest.Validate(manifestPath, mustLoadManifest(manifestPath))
					result.Checks = append(result.Checks, model.BaselineCheck{
						Name:   "manifest-file",
						Passed: validation.Valid,
						Actual: filepath.ToSlash(manifestPath),
						Detail: joinErrors(validation.Errors),
					})
					result.Ready = result.Ready && validation.Valid
				}
			} else {
				result.Ready = false
				result.Checks = append(result.Checks, model.BaselineCheck{
					Name:   "config-file",
					Passed: false,
					Detail: "workspace config not found; run `forktool init` first",
				})
			}

			for _, check := range result.Checks {
				result.Ready = result.Ready && check.Passed
			}

			return cliui.WriteJSON(cmd.OutOrStdout(), result)
		},
	}
}

func checkDirectory(name, path string) model.BaselineCheck {
	info, err := os.Stat(path)
	if err != nil {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: err.Error(),
		}
	}
	if !info.IsDir() {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: "path exists but is not a directory",
		}
	}
	return model.BaselineCheck{
		Name:   name,
		Passed: true,
		Actual: filepath.ToSlash(path),
	}
}

func mustLoadManifest(path string) model.FeatureManifest {
	loaded, err := manifest.Load(path)
	if err != nil {
		return model.FeatureManifest{}
	}
	return loaded
}

func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	return errors[0]
}
