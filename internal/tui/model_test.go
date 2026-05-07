package tui

import (
	"testing"

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
