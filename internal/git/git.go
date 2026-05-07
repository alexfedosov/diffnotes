package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type SourceKind string

const (
	SourceUnstaged SourceKind = "unstaged"
	SourceCommit   SourceKind = "commit"
)

type Source struct {
	Kind     SourceKind
	ID       string
	Title    string
	Subtitle string
}

func DiscoverRepo(path string) (string, error) {
	out, err := runGit(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func ListSources(repo string, commitLimit int) ([]Source, error) {
	if commitLimit < 1 {
		commitLimit = 1
	}

	var sources []Source
	changed, err := changedFileCount(repo)
	if err != nil {
		return nil, err
	}
	sources = append(sources, Source{
		Kind:     SourceUnstaged,
		ID:       "unstaged",
		Title:    "Unstaged changes",
		Subtitle: pluralize(changed, "file"),
	})

	log, err := runGit(repo, "log", "--date=relative", "--pretty=format:%H%x1f%h%x1f%cr%x1f%s", "-n", strconv.Itoa(commitLimit))
	if err != nil {
		return sources, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) != 4 {
			continue
		}
		sources = append(sources, Source{
			Kind:     SourceCommit,
			ID:       parts[0],
			Title:    parts[1] + " " + parts[3],
			Subtitle: parts[2],
		})
	}

	return sources, nil
}

func Diff(repo string, source Source) (string, error) {
	switch source.Kind {
	case SourceUnstaged:
		return runGit(repo, "diff", "--no-color", "--no-ext-diff", "--find-renames", "--find-copies", "--unified=3", "--")
	case SourceCommit:
		return runGit(repo, "show", "--format=", "--no-color", "--no-ext-diff", "--find-renames", "--find-copies", "--unified=3", source.ID, "--")
	default:
		return "", fmt.Errorf("unsupported source kind %q", source.Kind)
	}
}

func changedFileCount(repo string) (int, error) {
	out, err := runGit(repo, "diff", "--name-only", "--")
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}

func runGit(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}
	return stdout.String(), nil
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
