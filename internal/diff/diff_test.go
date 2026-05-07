package diff

import "testing"

func TestParseUnifiedDiffLines(t *testing.T) {
	raw := `diff --git a/app.go b/app.go
index 1111111..2222222 100644
--- a/app.go
+++ b/app.go
@@ -1,3 +1,4 @@
 package main
-old
+new
+extra
 unchanged
`

	files := Parse(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].DisplayPath() != "app.go" {
		t.Fatalf("unexpected path %q", files[0].DisplayPath())
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(files[0].Hunks))
	}

	lines := files[0].Hunks[0].Lines
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	if lines[1].Kind != Delete || lines[1].OldLine != 2 || lines[1].Anchor.Side != "old" {
		t.Fatalf("delete line was not anchored correctly: %#v", lines[1])
	}
	if lines[2].Kind != Add || lines[2].NewLine != 2 || lines[2].Anchor.Side != "new" {
		t.Fatalf("add line was not anchored correctly: %#v", lines[2])
	}
	if lines[4].Kind != Context || lines[4].OldLine != 3 || lines[4].NewLine != 4 {
		t.Fatalf("context line was not anchored correctly: %#v", lines[4])
	}
}

func TestParseNewFileDiff(t *testing.T) {
	raw := `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+hello
+world
`

	files := Parse(raw)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].DisplayPath() != "new.txt" {
		t.Fatalf("unexpected path %q", files[0].DisplayPath())
	}
	if files[0].Status() != "added" {
		t.Fatalf("unexpected status %q", files[0].Status())
	}
	lines := files[0].Hunks[0].Lines
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Kind != Add || lines[0].NewLine != 1 || lines[0].Anchor.File != "new.txt" {
		t.Fatalf("add line was not anchored correctly: %#v", lines[0])
	}
}
