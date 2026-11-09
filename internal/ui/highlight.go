package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	terminu "github.com/vulcanshen/terminu-go"
)

// Syntax colour for a code block, and for the text documents webu draws as
// one — JSON, YAML, TOML, Markdown, XML, JavaScript, CSS — is the family's:
// terminu.Syntax (tdp file-picker.3, note.2), catppuccin-mocha's colours,
// bold, italic and underline, with the gaps chroma leaves filled one way,
// so that code looks the same here as in every family member's preview and
// viewer, catu's among them (user 2026-11-09; webu had a palette of its
// own). Each line comes back in colour, and its runs become segments of
// the code kind, each with its ink and its bold, italic and underline.

// guessLang is chroma's guess at what language text is in, for a code
// block the page did not name (docs/reader.md); "" when it cannot say.
func guessLang(text string) string {
	if l := lexers.Analyse(text); l != nil {
		return strings.ToLower(l.Config().Name)
	}
	return ""
}

// syntaxLines is text in lang's syntax colours, one segment list per line;
// a lang that names no language leaves it plain, in Text. An escape in
// the page's own text is not taken for colour: it goes through as a
// stand-in and comes back as itself, for the row to name (tdp
// components.5).
func syntaxLines(lang, text string) [][]seg {
	esc := escStandIn(text)
	if esc == "" {
		return nil
	}
	lines := terminu.Syntax(lang, strings.Split(strings.ReplaceAll(text, "\x1b", esc), "\n"))
	out := make([][]seg, len(lines))
	for i, l := range lines {
		out[i] = sgrRuns(l)
		for k := range out[i] {
			out[i][k].text = strings.ReplaceAll(out[i][k].text, esc, "\x1b")
		}
	}
	return out
}

// escStandIn is a character text does not have, to carry its escapes
// through the colouring; "" when it has them all.
func escStandIn(text string) string {
	for _, c := range []string{"\uFFFC", "\uFFFD", "\uE000"} {
		if !strings.Contains(text, c) {
			return c
		}
	}
	return ""
}

var sgrSeq = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// sgrRuns is a line in colour as segments of the code kind: each run of
// one look, its ink and its bold, italic and underline.
func sgrRuns(line string) []seg {
	var out []seg
	cur := seg{item: -1, kind: segCode}
	add := func(s string) {
		if s == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].ink == cur.ink && out[n-1].attr == cur.attr {
			out[n-1].text += s
			return
		}
		run := cur
		run.text = s
		out = append(out, run)
	}
	for line != "" {
		at := sgrSeq.FindStringIndex(line)
		if at == nil {
			add(line)
			break
		}
		add(line[:at[0]])
		cur = applySGR(cur, line[at[0]+2:at[1]-1])
		line = line[at[1]:]
	}
	return out
}

// applySGR is s after one SGR sequence's parameters, as Syntax writes
// them: a 24-bit colour, bold, italic, underline and their resets.
func applySGR(s seg, params string) seg {
	ps := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	if len(ps) == 0 {
		ps = []string{"0"}
	}
	n := func(i int) int {
		if i >= len(ps) {
			return -1
		}
		v, _ := strconv.Atoi(ps[i])
		return v
	}
	for i := 0; i < len(ps); i++ {
		switch p := n(i); {
		case p == 0:
			s.ink, s.attr = "", 0
		case p == 1:
			s.attr |= attrBold
		case p == 3:
			s.attr |= attrItalic
		case p == 4:
			s.attr |= attrUnderline
		case p == 22:
			s.attr &^= attrBold
		case p == 23:
			s.attr &^= attrItalic
		case p == 24:
			s.attr &^= attrUnderline
		case p == 39:
			s.ink = ""
		case (p == 38 || p == 48) && n(i+1) == 2:
			if p == 38 && i+4 < len(ps) {
				s.ink = lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", n(i+2), n(i+3), n(i+4)))
			}
			i += 4
		case (p == 38 || p == 48) && n(i+1) == 5:
			i += 2
		}
	}
	return s
}

// langFor is the chroma lexer name for a document's content type, or ""
// for one that gets no colour (plain text, CSV).
func langFor(contentType string) string {
	ct := strings.ToLower(contentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case ct == "application/json", strings.HasSuffix(ct, "+json"):
		return "json"
	case strings.Contains(ct, "yaml"):
		return "yaml"
	case strings.Contains(ct, "toml"):
		return "toml"
	case ct == "text/markdown":
		return "markdown"
	case ct == "application/xml", ct == "text/xml", strings.HasSuffix(ct, "+xml"):
		return "xml"
	case strings.Contains(ct, "javascript"), strings.Contains(ct, "ecmascript"):
		return "javascript"
	case ct == "text/css":
		return "css"
	}
	return ""
}
