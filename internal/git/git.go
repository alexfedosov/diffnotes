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
	SourceUnstaged  SourceKind = "unstaged"
	SourceStaged    SourceKind = "staged"
	SourceUntracked SourceKind = "untracked"
	SourceCommit    SourceKind = "commit"
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
	staged, err := changedFileCount(repo, true)
	if err != nil {
		return nil, err
	}
	if staged > 0 {
		sources = append(sources, Source{
			Kind:     SourceStaged,
			ID:       "staged",
			Title:    "Staged changes",
			Subtitle: pluralize(staged, "file"),
		})
	}

	unstaged, err := changedFileCount(repo, false)
	if err != nil {
		return nil, err
	}
	if unstaged > 0 {
		sources = append(sources, Source{
			Kind:     SourceUnstaged,
			ID:       "unstaged",
			Title:    "Unstaged changes",
			Subtitle: pluralize(unstaged, "file"),
		})
	}

	untracked, err := untrackedFiles(repo)
	if err != nil {
		return nil, err
	}
	if len(untracked) > 0 {
		sources = append(sources, Source{
			Kind:     SourceUntracked,
			ID:       "untracked",
			Title:    "Untracked files",
			Subtitle: pluralize(len(untracked), "file"),
		})
	}

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
	case SourceStaged:
		return runGit(repo, "diff", "--cached", "--no-color", "--no-ext-diff", "--find-renames", "--find-copies", "--unified=3", "--")
	case SourceUnstaged:
		return runGit(repo, "diff", "--no-color", "--no-ext-diff", "--find-renames", "--find-copies", "--unified=3", "--")
	case SourceUntracked:
		return untrackedDiff(repo)
	case SourceCommit:
		return runGit(repo, "show", "--format=", "--no-color", "--no-ext-diff", "--find-renames", "--find-copies", "--unified=3", source.ID, "--")
	default:
		return "", fmt.Errorf("unsupported source kind %q", source.Kind)
	}
}

func changedFileCount(repo string, staged bool) (int, error) {
	args := []string{"diff", "--name-only"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	out, err := runGit(repo, args...)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}

func untrackedFiles(repo string) ([]string, error) {
	out, err := runGit(repo, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	parts := strings.Split(out, "\x00")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			files = append(files, part)
		}
	}
	return files, nil
}

func untrackedDiff(repo string) (string, error) {
	files, err := untrackedFiles(repo)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, file := range files {
		out, err := runGitAllowDiffExit(repo, "diff", "--no-index", "--no-color", "--unified=3", "--", "/dev/null", file)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
		if !strings.HasSuffix(out, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
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

func runGitAllowDiffExit(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok && stdout.Len() > 0 {
			return stdout.String(), nil
		}
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
