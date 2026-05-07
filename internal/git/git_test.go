package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSourcesShowsStagedAndUntrackedFiles(t *testing.T) {
	repo := t.TempDir()
	run(t, repo, "git", "init")

	writeFile(t, filepath.Join(repo, "staged.txt"), "staged\n")
	run(t, repo, "git", "add", "staged.txt")
	writeFile(t, filepath.Join(repo, "untracked.txt"), "untracked\n")

	sources, err := ListSources(repo, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(sources) < 2 {
		t.Fatalf("expected staged and untracked sources, got %#v", sources)
	}
	if sources[0].Kind != SourceStaged {
		t.Fatalf("expected staged source first, got %#v", sources[0])
	}
	if sources[1].Kind != SourceUntracked {
		t.Fatalf("expected untracked source second, got %#v", sources[1])
	}

	stagedDiff, err := Diff(repo, sources[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stagedDiff, "diff --git a/staged.txt b/staged.txt") {
		t.Fatalf("staged diff did not include staged file:\n%s", stagedDiff)
	}

	untrackedDiff, err := Diff(repo, sources[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(untrackedDiff, "diff --git a/untracked.txt b/untracked.txt") {
		t.Fatalf("untracked diff did not include untracked file:\n%s", untrackedDiff)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
