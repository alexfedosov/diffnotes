package diff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type LineKind int

const (
	Context LineKind = iota
	Add
	Delete
	Meta
)

type Anchor struct {
	File string
	Side string
	Line int
}

type Line struct {
	Kind    LineKind
	Content string
	OldLine int
	NewLine int
	Anchor  Anchor
}

type Hunk struct {
	Header string
	Lines  []Line
}

type File struct {
	Header  string
	OldPath string
	NewPath string
	Hunks   []Hunk
}

func (f File) DisplayPath() string {
	switch {
	case f.NewPath != "" && f.NewPath != "/dev/null":
		return f.NewPath
	case f.OldPath != "" && f.OldPath != "/dev/null":
		return f.OldPath
	default:
		return f.Header
	}
}

func (f File) Status() string {
	switch {
	case f.OldPath == "/dev/null":
		return "added"
	case f.NewPath == "/dev/null":
		return "deleted"
	case f.OldPath != "" && f.NewPath != "" && f.OldPath != f.NewPath:
		return "renamed"
	default:
		return "modified"
	}
}

type RowType int

const (
	RowFile RowType = iota
	RowHunk
	RowLine
)

type Row struct {
	Type      RowType
	FileIndex int
	HunkIndex int
	LineIndex int
	Header    string
	Line      *Line
	File      *File
}

var hunkPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func Parse(raw string) []File {
	lines := strings.Split(raw, "\n")

	var files []File
	var current *File
	var oldLine, newLine int

	flush := func() {
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}

	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}

		if strings.HasPrefix(line, "diff --git ") {
			flush()
			oldPath, newPath := parseDiffHeader(line)
			current = &File{Header: line, OldPath: oldPath, NewPath: newPath}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			current.OldPath = parsePath(line[4:])
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			current.NewPath = parsePath(line[4:])
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			oldLine, newLine = parseHunkStart(line)
			current.Hunks = append(current.Hunks, Hunk{Header: line})
			continue
		}

		if len(current.Hunks) == 0 {
			continue
		}

		hunk := &current.Hunks[len(current.Hunks)-1]
		hunk.Lines = append(hunk.Lines, parseLine(line, current.DisplayPath(), &oldLine, &newLine))
	}

	flush()
	return files
}

func Flatten(files []File) []Row {
	var rows []Row
	for fi := range files {
		file := &files[fi]
		rows = append(rows, Row{
			Type:      RowFile,
			FileIndex: fi,
			Header:    fmt.Sprintf("%s  (%s)", file.DisplayPath(), file.Status()),
			File:      file,
		})
		for hi := range file.Hunks {
			hunk := &file.Hunks[hi]
			rows = append(rows, Row{
				Type:      RowHunk,
				FileIndex: fi,
				HunkIndex: hi,
				Header:    hunk.Header,
				File:      file,
			})
			for li := range hunk.Lines {
				line := &hunk.Lines[li]
				rows = append(rows, Row{
					Type:      RowLine,
					FileIndex: fi,
					HunkIndex: hi,
					LineIndex: li,
					Line:      line,
					File:      file,
				})
			}
		}
	}
	return rows
}

func parseLine(text string, file string, oldLine *int, newLine *int) Line {
	if text == `\ No newline at end of file` {
		return Line{Kind: Meta, Content: text}
	}
	if text == "" {
		return Line{Kind: Context, OldLine: *oldLine, NewLine: *newLine, Anchor: Anchor{File: file, Side: "new", Line: *newLine}}
	}

	switch text[0] {
	case '+':
		line := Line{
			Kind:    Add,
			Content: text[1:],
			NewLine: *newLine,
			Anchor:  Anchor{File: file, Side: "new", Line: *newLine},
		}
		*newLine++
		return line
	case '-':
		line := Line{
			Kind:    Delete,
			Content: text[1:],
			OldLine: *oldLine,
			Anchor:  Anchor{File: file, Side: "old", Line: *oldLine},
		}
		*oldLine++
		return line
	case ' ':
		line := Line{
			Kind:    Context,
			Content: text[1:],
			OldLine: *oldLine,
			NewLine: *newLine,
			Anchor:  Anchor{File: file, Side: "new", Line: *newLine},
		}
		*oldLine++
		*newLine++
		return line
	default:
		return Line{Kind: Meta, Content: text}
	}
}

func parseHunkStart(header string) (int, int) {
	match := hunkPattern.FindStringSubmatch(header)
	if match == nil {
		return 0, 0
	}
	oldStart, _ := strconv.Atoi(match[1])
	newStart, _ := strconv.Atoi(match[3])
	return oldStart, newStart
}

func parseDiffHeader(header string) (string, string) {
	parts := strings.Fields(strings.TrimPrefix(header, "diff --git "))
	if len(parts) < 2 {
		return "", ""
	}
	return parsePath(parts[0]), parsePath(parts[1])
}

func parsePath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "\"") {
		if unquoted, err := strconv.Unquote(path); err == nil {
			path = unquoted
		}
	}
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}
