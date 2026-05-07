package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type Segment struct {
	Text      string
	Color     string
	Bold      bool
	Italic    bool
	Underline bool
}

const defaultMaxEntries = 10000

type Highlighter struct {
	style      *chroma.Style
	lexers     map[string]lexerEntry
	lines      map[string][]Segment
	lineOrder  []string
	maxEntries int
}

type lexerEntry struct {
	lexer chroma.Lexer
	found bool
}

var defaultHighlighter = NewHighlighter()

func Highlight(path string, source string) []Segment {
	return defaultHighlighter.Highlight(path, source)
}

func NewHighlighter() *Highlighter {
	return &Highlighter{
		style:      styles.Get("github-dark"),
		lexers:     make(map[string]lexerEntry),
		lines:      make(map[string][]Segment),
		maxEntries: defaultMaxEntries,
	}
}

func (h *Highlighter) Highlight(path string, source string) []Segment {
	if source == "" {
		return []Segment{{Text: source}}
	}
	if h == nil {
		h = defaultHighlighter
	}

	key := path + "\x00" + source
	if segments, ok := h.lines[key]; ok {
		return segments
	}

	lexer := h.lexerFor(path)
	if lexer == nil {
		return []Segment{{Text: source}}
	}

	var (
		segments []Segment
		ok       bool
	)
	func() {
		defer func() {
			if recover() != nil {
				segments = nil
			}
		}()

		iterator, err := lexer.Tokenise(nil, source)
		if err != nil {
			return
		}

		for token := iterator(); token != chroma.EOF; token = iterator() {
			if token.Value == "" {
				continue
			}
			entry := h.style.Get(token.Type)
			segment := Segment{Text: strings.TrimRight(token.Value, "\n")}
			if entry.Colour.IsSet() {
				segment.Color = entry.Colour.String()
			}
			segment.Bold = entry.Bold == chroma.Yes
			segment.Italic = entry.Italic == chroma.Yes
			segment.Underline = entry.Underline == chroma.Yes
			segments = append(segments, segment)
		}
		ok = true
	}()

	if !ok || len(segments) == 0 {
		return []Segment{{Text: source}}
	}
	segments = mergeAdjacent(segments)
	h.remember(key, segments)
	return segments
}

func PlainText(segments []Segment) string {
	var builder strings.Builder
	for _, segment := range segments {
		builder.WriteString(segment.Text)
	}
	return builder.String()
}

func (h *Highlighter) lexerFor(path string) chroma.Lexer {
	if entry, ok := h.lexers[path]; ok {
		if entry.found {
			return entry.lexer
		}
		return nil
	}

	lexer := lexers.Match(path)
	if lexer == nil {
		h.lexers[path] = lexerEntry{}
		return nil
	}
	lexer = chroma.Coalesce(lexer)
	h.lexers[path] = lexerEntry{lexer: lexer, found: true}
	return lexer
}

func (h *Highlighter) remember(key string, segments []Segment) {
	if h.maxEntries <= 0 {
		return
	}
	if _, ok := h.lines[key]; ok {
		h.lines[key] = segments
		return
	}
	if len(h.lines) >= h.maxEntries && len(h.lineOrder) > 0 {
		oldest := h.lineOrder[0]
		h.lineOrder = h.lineOrder[1:]
		delete(h.lines, oldest)
	}
	h.lines[key] = segments
	h.lineOrder = append(h.lineOrder, key)
}

func mergeAdjacent(segments []Segment) []Segment {
	if len(segments) < 2 {
		return segments
	}

	merged := make([]Segment, 0, len(segments))
	for _, segment := range segments {
		if segment.Text == "" {
			continue
		}
		last := len(merged) - 1
		if last >= 0 && sameStyle(merged[last], segment) {
			merged[last].Text += segment.Text
			continue
		}
		merged = append(merged, segment)
	}
	if len(merged) == 0 {
		return []Segment{{}}
	}
	return merged
}

func sameStyle(a Segment, b Segment) bool {
	return a.Color == b.Color &&
		a.Bold == b.Bold &&
		a.Italic == b.Italic &&
		a.Underline == b.Underline
}
