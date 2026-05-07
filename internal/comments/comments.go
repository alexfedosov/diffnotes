package comments

import (
	"fmt"
	"sort"
	"strings"
)

type Note struct {
	ID             string
	SourceID       string
	SourceTitle    string
	SourceSubtitle string
	File           string
	Side           string
	Line           int
	Message        string
	Code           string
}

type Store struct {
	notes map[string]Note
}

func NewStore() *Store {
	return &Store{notes: make(map[string]Note)}
}

func NoteID(sourceID, file, side string, line int) string {
	return fmt.Sprintf("%s|%s|%s|%d", sourceID, file, side, line)
}

func (s *Store) Upsert(note Note) {
	if note.ID == "" {
		note.ID = NoteID(note.SourceID, note.File, note.Side, note.Line)
	}
	s.notes[note.ID] = note
}

func (s *Store) Delete(id string) bool {
	if _, ok := s.notes[id]; !ok {
		return false
	}
	delete(s.notes, id)
	return true
}

func (s *Store) Get(id string) (Note, bool) {
	note, ok := s.notes[id]
	return note, ok
}

func (s *Store) Len() int {
	return len(s.notes)
}

func (s *Store) List() []Note {
	out := make([]Note, 0, len(s.notes))
	for _, note := range s.notes {
		out = append(out, note)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.SourceTitle != b.SourceTitle {
			return a.SourceTitle < b.SourceTitle
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Side < b.Side
	})
	return out
}

func Format(notes []Note) string {
	var b strings.Builder
	b.WriteString("Review comments for coding agent\n")

	lastSource := ""
	for _, note := range notes {
		if note.SourceTitle != lastSource {
			if lastSource != "" {
				b.WriteByte('\n')
			}
			b.WriteString("\nSource: ")
			b.WriteString(note.SourceTitle)
			if note.SourceSubtitle != "" {
				b.WriteString(" (")
				b.WriteString(note.SourceSubtitle)
				b.WriteString(")")
			}
			b.WriteByte('\n')
			lastSource = note.SourceTitle
		}

		b.WriteString("- ")
		b.WriteString(note.File)
		b.WriteByte(':')
		b.WriteString(fmt.Sprintf("%d", note.Line))
		if note.Side != "" {
			b.WriteString(" [")
			b.WriteString(note.Side)
			b.WriteByte(']')
		}
		b.WriteByte('\n')
		b.WriteString("  Message: ")
		b.WriteString(note.Message)
		b.WriteByte('\n')
		if note.Code != "" {
			b.WriteString("  Code: ")
			b.WriteString(strings.TrimSpace(note.Code))
			b.WriteByte('\n')
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
