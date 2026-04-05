package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ValidateRepositoryPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("repository path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("repository path not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repository path is not a directory")
	}
	return nil
}

func IsRepository(ctx context.Context, path string) error {
	output, err := run(ctx, path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "true" {
		return fmt.Errorf("path is not a git work tree")
	}
	return nil
}

func GetRemoteURL(ctx context.Context, path, remoteName string) (string, error) {
	return run(ctx, path, "remote", "get-url", remoteName)
}

func ResolveRevision(ctx context.Context, path, revision string) (string, error) {
	return run(ctx, path, "rev-parse", revision+"^{commit}")
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}

	return strings.TrimSpace(stdout.String()), nil
}
