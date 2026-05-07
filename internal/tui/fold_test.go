package tui

import (
	"reflect"
	"testing"

	"github.com/alexfedosov/diffnotes/internal/comments"
	"github.com/alexfedosov/diffnotes/internal/diff"
)

func TestFoldedRowsKeepFilesAndThreeLinesBeforeComments(t *testing.T) {
	model := NewModel(".", 10)
	model.loadedSourceID = "source"
	model.commentsOnly = true
	model.rows = []diff.Row{
		fileRow(0, "a.go"),
		lineRow(0, "a.go", 1),
		lineRow(0, "a.go", 2),
		lineRow(0, "a.go", 3),
		lineRow(0, "a.go", 4),
		lineRow(0, "a.go", 5),
		fileRow(1, "b.go"),
		lineRow(1, "b.go", 1),
		lineRow(1, "b.go", 2),
	}
	model.notes = comments.NewStore()
	model.notes.Upsert(comments.Note{
		ID:       comments.NoteID("source", "a.go", "new", 4),
		SourceID: "source",
		File:     "a.go",
		Side:     "new",
		Line:     4,
		Message:  "comment on a",
	})
	model.notes.Upsert(comments.Note{
		ID:       comments.NoteID("source", "b.go", "new", 2),
		SourceID: "source",
		File:     "b.go",
		Side:     "new",
		Line:     2,
		Message:  "comment on b",
	})

	got := model.foldedRows()
	want := []int{0, 1, 2, 3, 4, 6, 7, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("folded rows mismatch\ngot  %#v\nwant %#v", got, want)
	}
}

func fileRow(fileIndex int, path string) diff.Row {
	file := &diff.File{NewPath: path}
	return diff.Row{
		Type:      diff.RowFile,
		FileIndex: fileIndex,
		Header:    path,
		File:      file,
	}
}

func lineRow(fileIndex int, path string, line int) diff.Row {
	file := &diff.File{NewPath: path}
	diffLine := &diff.Line{
		Kind:    diff.Context,
		Content: "line",
		NewLine: line,
		Anchor:  diff.Anchor{File: path, Side: "new", Line: line},
	}
	return diff.Row{
		Type:      diff.RowLine,
		FileIndex: fileIndex,
		Line:      diffLine,
		File:      file,
	}
}
