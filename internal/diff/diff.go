package diff

import (
	"fmt"
	"strconv"
	"strings"

	sourcediff "github.com/sourcegraph/go-diff/diff"
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

func Parse(raw string) []File {
	parsed, err := sourcediff.ParseMultiFileDiff([]byte(raw))
	if err != nil {
		return nil
	}

	files := make([]File, 0, len(parsed))
	for _, parsedFile := range parsed {
		file := File{
			Header:  fileHeader(parsedFile),
			OldPath: parsePath(parsedFile.OrigName),
			NewPath: parsePath(parsedFile.NewName),
		}
		for _, parsedHunk := range parsedFile.Hunks {
			oldLine := int(parsedHunk.OrigStartLine)
			newLine := int(parsedHunk.NewStartLine)
			hunk := Hunk{Header: hunkHeader(parsedHunk)}
			for _, line := range hunkBodyLines(parsedHunk.Body) {
				hunk.Lines = append(hunk.Lines, parseLine(line, file.DisplayPath(), &oldLine, &newLine))
			}
			file.Hunks = append(file.Hunks, hunk)
		}
		files = append(files, file)
	}

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

func fileHeader(file *sourcediff.FileDiff) string {
	if len(file.Extended) > 0 {
		return file.Extended[0]
	}
	return fmt.Sprintf("diff --git %s %s", file.OrigName, file.NewName)
}

func hunkHeader(hunk *sourcediff.Hunk) string {
	header := fmt.Sprintf("@@ %s %s @@", hunkRange("-", hunk.OrigStartLine, hunk.OrigLines), hunkRange("+", hunk.NewStartLine, hunk.NewLines))
	if hunk.Section != "" {
		header += " " + hunk.Section
	}
	return header
}

func hunkRange(prefix string, start int32, count int32) string {
	if count == 1 {
		return fmt.Sprintf("%s%d", prefix, start)
	}
	return fmt.Sprintf("%s%d,%d", prefix, start, count)
}

func hunkBodyLines(body []byte) []string {
	lines := strings.Split(string(body), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
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
