package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
			result.Checks = append(result.Checks, checkDirectory("baselines-dir", filepath.Join(workspaceDir, "baselines")))

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

				decisionPath := resolveWorkspacePath(workspace.RepoRoot, workspace.Config.DecisionFile.Path)
				result.Checks = append(result.Checks, checkFile("decision-file", decisionPath))
				result.Checks = append(result.Checks, checkDirectory("output-dir", resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Output.Dir)))
				result.Checks = append(result.Checks, checkConfiguredRepo("local-repo", resolveWorkspacePath(workspace.RepoRoot, workspace.Config.LocalRepo.Path)))
				result.Checks = append(result.Checks, checkConfiguredRepo("official-repo", resolveWorkspacePath(workspace.RepoRoot, workspace.Config.OfficialRepo.Path)))
				result.Checks = append(result.Checks, checkBaselineInputs(workspace.Config))

				localRepoRoot := resolveWorkspacePath(workspace.RepoRoot, workspace.Config.LocalRepo.Path)
				if localRepoRoot != "" {
					result.Checks = append(result.Checks, checkOptionalDirectory("backend-dir", filepath.Join(localRepoRoot, "backend")))
					result.Checks = append(result.Checks, checkOptionalDirectory("frontend-dir", filepath.Join(localRepoRoot, "frontend")))
					result.Checks = append(result.Checks, checkOptionalDirectory("deploy-dir", filepath.Join(localRepoRoot, "deploy")))
					result.Checks = append(result.Checks, checkOptionalDirectory("docs-dir", filepath.Join(localRepoRoot, "docs")))
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
	if strings.TrimSpace(path) == "" {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: "path is empty",
		}
	}
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

func checkOptionalDirectory(name, path string) model.BaselineCheck {
	if strings.TrimSpace(path) == "" {
		return model.BaselineCheck{Name: name, Passed: false, Detail: "path is empty"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: err.Error(),
		}
	}
	return model.BaselineCheck{
		Name:   name,
		Passed: info.IsDir(),
		Actual: filepath.ToSlash(path),
	}
}

func checkFile(name, path string) model.BaselineCheck {
	if strings.TrimSpace(path) == "" {
		return model.BaselineCheck{Name: name, Passed: false, Detail: "path is empty"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: err.Error(),
		}
	}
	if info.IsDir() {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: "path exists but is a directory",
		}
	}
	return model.BaselineCheck{
		Name:   name,
		Passed: true,
		Actual: filepath.ToSlash(path),
	}
}

func checkConfiguredRepo(name, path string) model.BaselineCheck {
	if strings.TrimSpace(path) == "" {
		return model.BaselineCheck{
			Name:   name,
			Passed: false,
			Detail: "repo path is empty",
		}
	}
	return checkDirectory(name, path)
}

func checkBaselineInputs(config model.WorkspaceConfig) model.BaselineCheck {
	remoteConfigured := strings.TrimSpace(config.OfficialRepo.RemoteURL) != ""
	revisionConfigured := strings.TrimSpace(config.OfficialRepo.Tag) != "" || strings.TrimSpace(config.OfficialRepo.Commit) != ""

	if remoteConfigured && revisionConfigured {
		return model.BaselineCheck{
			Name:   "baseline-inputs",
			Passed: true,
			Detail: "official repo path, remote, and revision are configured",
		}
	}

	missing := make([]string, 0, 2)
	if !remoteConfigured {
		missing = append(missing, "remoteUrl")
	}
	if !revisionConfigured {
		missing = append(missing, "tag/commit")
	}
	return model.BaselineCheck{
		Name:   "baseline-inputs",
		Passed: false,
		Detail: "missing " + strings.Join(missing, ", "),
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
