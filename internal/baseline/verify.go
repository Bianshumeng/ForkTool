package baseline

import (
	"context"
	"fmt"
	"strings"

	gitintegration "forktool/internal/integrations/git"
	"forktool/pkg/model"
)

type VerifyInput struct {
	Official   model.RepoConfig
	RemoteName string
}

func Verify(ctx context.Context, input VerifyInput) (model.BaselineVerificationResult, error) {
	remoteName := strings.TrimSpace(input.RemoteName)
	if remoteName == "" {
		remoteName = "origin"
	}

	result := model.BaselineVerificationResult{
		Official: model.RepoSnapshot{
			Path:   input.Official.Path,
			Kind:   input.Official.Kind,
			Tag:    input.Official.Tag,
			Commit: input.Official.Commit,
		},
		RemoteName:        remoteName,
		ExpectedRemoteURL: input.Official.RemoteURL,
		ExpectedTag:       input.Official.Tag,
		ExpectedCommit:    input.Official.Commit,
	}

	var errors []string

	if err := gitintegration.ValidateRepositoryPath(input.Official.Path); err != nil {
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:   "official-path",
			Passed: false,
			Detail: err.Error(),
		})
		errors = append(errors, err.Error())
		result.Errors = errors
		return result, nil
	}

	result.Checks = append(result.Checks, model.BaselineCheck{
		Name:   "official-path",
		Passed: true,
		Actual: input.Official.Path,
	})

	if err := gitintegration.IsRepository(ctx, input.Official.Path); err != nil {
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:   "git-repository",
			Passed: false,
			Detail: err.Error(),
		})
		errors = append(errors, err.Error())
		result.Errors = errors
		return result, nil
	}

	result.Checks = append(result.Checks, model.BaselineCheck{
		Name:   "git-repository",
		Passed: true,
	})

	actualRemoteURL, err := gitintegration.GetRemoteURL(ctx, input.Official.Path, remoteName)
	if err != nil {
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:   "remote-url",
			Passed: false,
			Detail: err.Error(),
		})
		errors = append(errors, err.Error())
	} else {
		result.ActualRemoteURL = actualRemoteURL
		result.Official.RemoteURL = actualRemoteURL
		switch {
		case strings.TrimSpace(input.Official.RemoteURL) == "":
			message := "expected remoteUrl is required for baseline verification"
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:   "remote-url",
				Passed: false,
				Actual: actualRemoteURL,
				Detail: message,
			})
			errors = append(errors, message)
		case normalizeRemoteURL(actualRemoteURL) != normalizeRemoteURL(input.Official.RemoteURL):
			message := fmt.Sprintf("remote URL mismatch: expected %q, got %q", input.Official.RemoteURL, actualRemoteURL)
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "remote-url",
				Passed:   false,
				Expected: input.Official.RemoteURL,
				Actual:   actualRemoteURL,
				Detail:   message,
			})
			errors = append(errors, message)
		default:
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "remote-url",
				Passed:   true,
				Expected: input.Official.RemoteURL,
				Actual:   actualRemoteURL,
			})
		}
	}

	if strings.TrimSpace(input.Official.Tag) == "" && strings.TrimSpace(input.Official.Commit) == "" {
		message := "either official tag or commit must be provided"
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:   "revision",
			Passed: false,
			Detail: message,
		})
		errors = append(errors, message)
	}

	if strings.TrimSpace(input.Official.Tag) != "" {
		resolvedTagCommit, err := gitintegration.ResolveRevision(ctx, input.Official.Path, input.Official.Tag)
		if err != nil {
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "tag",
				Passed:   false,
				Expected: input.Official.Tag,
				Detail:   err.Error(),
			})
			errors = append(errors, err.Error())
		} else {
			result.ResolvedTagCommit = resolvedTagCommit
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "tag",
				Passed:   true,
				Expected: input.Official.Tag,
				Actual:   resolvedTagCommit,
			})
		}
	}

	if strings.TrimSpace(input.Official.Commit) != "" {
		resolvedCommit, err := gitintegration.ResolveRevision(ctx, input.Official.Path, input.Official.Commit)
		if err != nil {
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "commit",
				Passed:   false,
				Expected: input.Official.Commit,
				Detail:   err.Error(),
			})
			errors = append(errors, err.Error())
		} else {
			result.ResolvedCommit = resolvedCommit
			result.Official.Commit = resolvedCommit
			result.Checks = append(result.Checks, model.BaselineCheck{
				Name:     "commit",
				Passed:   true,
				Expected: input.Official.Commit,
				Actual:   resolvedCommit,
			})
		}
	}

	if result.ResolvedCommit == "" && result.ResolvedTagCommit != "" {
		result.Official.Commit = result.ResolvedTagCommit
	}

	if result.ResolvedTagCommit != "" && result.ResolvedCommit != "" && result.ResolvedTagCommit != result.ResolvedCommit {
		message := fmt.Sprintf("tag %q resolves to %s but commit %q resolves to %s", input.Official.Tag, result.ResolvedTagCommit, input.Official.Commit, result.ResolvedCommit)
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:     "tag-commit-match",
			Passed:   false,
			Expected: result.ResolvedTagCommit,
			Actual:   result.ResolvedCommit,
			Detail:   message,
		})
		errors = append(errors, message)
	} else if result.ResolvedTagCommit != "" && result.ResolvedCommit != "" {
		result.Checks = append(result.Checks, model.BaselineCheck{
			Name:     "tag-commit-match",
			Passed:   true,
			Expected: result.ResolvedTagCommit,
			Actual:   result.ResolvedCommit,
		})
	}

	result.Errors = errors
	result.Valid = len(errors) == 0
	return result, nil
}

func normalizeRemoteURL(value string) string {
	return strings.TrimSpace(strings.TrimSuffix(value, ".git"))
}
