package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestCommentCompletionAcceptsWithTab(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("sele")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.input.Value() != "selectedSyntaxColor" {
		t.Fatalf("unexpected completed input %q", got.input.Value())
	}
	if got.status != "completed selectedSyntaxColor" {
		t.Fatalf("unexpected completion status %q", got.status)
	}
}

func TestCommentCompletionAcceptsWithCtrlY(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("strings.To")
	model.selectedRow = 3

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	got := updated.(Model)
	if got.input.Value() != "strings.ToLower" {
		t.Fatalf("unexpected completed input %q", got.input.Value())
	}
}

func TestCommentCompletionSelectionCyclesWithCtrlNAndCtrlP(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("sele")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	got := updated.(Model)
	if got.status != "completion 2/2: selectedStyle" {
		t.Fatalf("unexpected next completion status %q", got.status)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(Model)
	if got.input.Value() != "selectedStyle" {
		t.Fatalf("unexpected completed input after ctrl+n %q", got.input.Value())
	}

	model = completionTestModel()
	model.startEdit()
	model.input.InsertString("sele")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	got = updated.(Model)
	if got.status != "completion 2/2: selectedStyle" {
		t.Fatalf("unexpected previous completion status %q", got.status)
	}
}

func TestEscClosesCommentCompletionBeforeCancelingEditor(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("sele")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.mode != modeEditing {
		t.Fatal("first esc canceled the editor instead of closing completion")
	}
	if got.status != "completion closed" {
		t.Fatalf("unexpected status after closing completion %q", got.status)
	}
	if _, ok := got.currentCompletion(); ok {
		t.Fatal("completion stayed visible after esc")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.mode != modeNormal {
		t.Fatal("second esc did not cancel the editor")
	}
	if got.status != "comment canceled" {
		t.Fatalf("unexpected status after canceling editor %q", got.status)
	}
}

func TestCommentCompletionStaysWithinCurrentHunk(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("unrel")

	if completion, ok := model.currentCompletion(); ok {
		t.Fatalf("completion crossed hunk boundary: %#v", completion)
	}
}

func TestToggleSplitView(t *testing.T) {
	model := NewModel(".", 10)
	model.width = 120
	model.height = 30

	if model.splitView {
		t.Fatal("expected split view to be disabled by default")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(Model)
	if !got.splitView {
		t.Fatal("expected split view to be enabled after toggling")
	}
	if got.status != "split view enabled" {
		t.Fatalf("unexpected status after enabling split view: %q", got.status)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got = updated.(Model)
	if got.splitView {
		t.Fatal("expected split view to be disabled after toggling again")
	}
	if got.status != "unified view enabled" {
		t.Fatalf("unexpected status after disabling split view: %q", got.status)
	}
}

func TestToggleSidebarVisibility(t *testing.T) {
	model := NewModel(".", 10)
	model.width = 120
	model.height = 30
	model.focus = focusSources

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := updated.(Model)
	if !got.sidebarHidden {
		t.Fatal("expected sidebar to be hidden after toggling")
	}
	if got.focus != focusDiff {
		t.Fatal("expected hiding sidebar to focus diff")
	}
	if got.status != "sidebar hidden" {
		t.Fatalf("unexpected status after hiding sidebar: %q", got.status)
	}
	if got.sidebarWidth() != 0 {
		t.Fatalf("hidden sidebar should have zero width, got %d", got.sidebarWidth())
	}
	if got.diffWidth() != got.width {
		t.Fatalf("hidden sidebar should give full width to diff, got %d want %d", got.diffWidth(), got.width)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(Model)
	if got.focus != focusDiff {
		t.Fatal("tab should keep focus on diff while sidebar is hidden")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got = updated.(Model)
	if got.focus != focusDiff {
		t.Fatal("h should not focus the hidden sidebar")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got = updated.(Model)
	if got.sidebarHidden {
		t.Fatal("expected sidebar to be shown after toggling again")
	}
	if got.status != "sidebar shown" {
		t.Fatalf("unexpected status after showing sidebar: %q", got.status)
	}
	if got.sidebarWidth() == 0 {
		t.Fatal("shown sidebar should have a non-zero width")
	}
}

func completionTestModel() Model {
	file := &diff.File{NewPath: "internal/tui/view.go"}
	model := NewModel(".", 10)
	model.width = 120
	model.height = 30
	model.focus = focusDiff
	model.loadedSourceID = "source"
	model.loadedSource = git.Source{ID: "source", Title: "Unstaged changes"}
	model.rows = []diff.Row{
		{Type: diff.RowFile, FileIndex: 0, Header: "internal/tui/view.go (modified)", File: file},
		{Type: diff.RowHunk, FileIndex: 0, Header: "@@ -1,4 +1,4 @@", File: file},
		{Type: diff.RowLine, FileIndex: 0, HunkIndex: 0, Line: &diff.Line{
			Kind:    diff.Context,
			Content: "func selectedSyntaxColor(selectedStyle string) string {",
			OldLine: 1,
			NewLine: 1,
			Anchor:  diff.Anchor{File: "internal/tui/view.go", Side: "new", Line: 1},
		}, File: file},
		{Type: diff.RowLine, FileIndex: 0, HunkIndex: 0, Line: &diff.Line{
			Kind:    diff.Context,
			Content: "switch strings.ToLower(color) {",
			OldLine: 2,
			NewLine: 2,
			Anchor:  diff.Anchor{File: "internal/tui/view.go", Side: "new", Line: 2},
		}, File: file},
		{Type: diff.RowHunk, FileIndex: 0, Header: "@@ -20,2 +20,2 @@", File: file},
		{Type: diff.RowLine, FileIndex: 0, HunkIndex: 1, Line: &diff.Line{
			Kind:    diff.Context,
			Content: "func unrelatedCompletion() {}",
			OldLine: 20,
			NewLine: 20,
			Anchor:  diff.Anchor{File: "internal/tui/view.go", Side: "new", Line: 20},
		}, File: file},
	}
	model.selectedRow = 2
	return model
}
