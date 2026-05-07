package tui

import (
	"errors"
	"testing"

	"github.com/alexfedosov/diffnotes/internal/comments"
	"github.com/alexfedosov/diffnotes/internal/diff"
	"github.com/alexfedosov/diffnotes/internal/git"
)

func TestStaleDiffLoadIsIgnored(t *testing.T) {
	model := NewModel(".", 10)
	model.diffRequest = 2
	model.loading = true
	model.loadedSourceID = "current"
	model.rows = []diff.Row{{Type: diff.RowFile, Header: "current"}}

	updated, _ := model.Update(diffLoadedMsg{
		request: 1,
		source:  git.Source{ID: "stale", Title: "stale"},
		rows:    []diff.Row{{Type: diff.RowFile, Header: "stale"}},
	})

	got := updated.(Model)
	if got.loadedSourceID != "current" {
		t.Fatalf("stale diff replaced loaded source: %q", got.loadedSourceID)
	}
	if !got.loading {
		t.Fatal("stale diff cleared loading state")
	}
	if got.rows[0].Header != "current" {
		t.Fatalf("stale diff replaced rows: %#v", got.rows)
	}
}

func TestCurrentDiffLoadIsApplied(t *testing.T) {
	model := NewModel(".", 10)
	model.diffRequest = 2
	model.loading = true

	updated, _ := model.Update(diffLoadedMsg{
		request: 2,
		source:  git.Source{ID: "current", Title: "current"},
		rows:    []diff.Row{{Type: diff.RowFile, Header: "current"}},
	})

	got := updated.(Model)
	if got.loadedSourceID != "current" {
		t.Fatalf("current diff was not applied: %q", got.loadedSourceID)
	}
	if got.loading {
		t.Fatal("current diff did not clear loading state")
	}
	if got.rows[0].Header != "current" {
		t.Fatalf("current diff did not replace rows: %#v", got.rows)
	}
}

func TestCopyNotesRunsThroughCommand(t *testing.T) {
	model := NewModel(".", 10)
	model.notes.Upsert(comments.Note{
		SourceID:    "source",
		SourceTitle: "source",
		File:        "main.go",
		Side:        "new",
		Line:        12,
		Message:     "check this",
	})

	previousWrite := clipboardWrite
	t.Cleanup(func() { clipboardWrite = previousWrite })
	var copied string
	clipboardWrite = func(text string) (string, error) {
		copied = text
		return "test clipboard", nil
	}

	cmd := model.copyNotes()
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	if model.status != "copying 1 comments" {
		t.Fatalf("unexpected pre-copy status %q", model.status)
	}

	msg := cmd()
	if copied == "" {
		t.Fatal("clipboard command did not receive formatted notes")
	}

	updated, _ := model.Update(msg)
	got := updated.(Model)
	if got.status != "copied 1 comments using test clipboard" {
		t.Fatalf("unexpected post-copy status %q", got.status)
	}
}

func TestCopyNotesCommandReportsErrors(t *testing.T) {
	model := NewModel(".", 10)

	updated, _ := model.Update(clipboardCopiedMsg{count: 1, err: errors.New("copy failed")})
	got := updated.(Model)
	if got.status != "copy failed" {
		t.Fatalf("unexpected copy error status %q", got.status)
	}
}
