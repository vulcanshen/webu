package ui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Syntax colour for the text documents webu draws as one code block —
// JSON, YAML, TOML, Markdown, XML, JavaScript, CSS — through chroma, the
// family's lexer (sshu and filu use it in their viewers). A token becomes a
// segment of one of the code kinds below; the colours are in segStyles.
//
// Only what is a different KIND of thing gets a colour: a key, a string,
// a number, a constant, a comment. Punctuation dims. Everything else is
// text. That keeps a JSON document readable as key / value at a glance
// without turning it into a Christmas tree.

// highlightLines lexes text as lang and returns one segment list per line.
// nil when there is no lexer for lang, and the caller draws plain.
func highlightLines(lang, text string) [][]seg {
	if lang == "" {
		return nil
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return nil
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil
	}
	data := lang == "json" || lang == "yaml" || lang == "toml"
	lines := [][]seg{{}}
	for tok := it(); tok != chroma.EOF; tok = it() {
		kind := tokenKind(tok.Type, data)
		parts := strings.Split(tok.Value, "\n")
		for i, p := range parts {
			if i > 0 {
				lines = append(lines, []seg{})
			}
			if p == "" {
				continue
			}
			last := len(lines) - 1
			lines[last] = append(lines[last], seg{text: p, item: -1, kind: kind})
		}
	}
	return lines
}

// tokenKind maps a chroma token type to a segment kind. In a data format
// every Name is a key (JSON: NameTag; TOML: NameOther; YAML: NameTag);
// in a program only the tag-like names are.
func tokenKind(t chroma.TokenType, data bool) segKind {
	switch {
	case t.InCategory(chroma.Comment):
		return segCodeComment
	case t.InSubCategory(chroma.LiteralString), t == chroma.LiteralDate:
		return segCodeString
	case t.InSubCategory(chroma.LiteralNumber):
		return segCodeNumber
	case t == chroma.KeywordConstant:
		return segCodeConst
	case t.InCategory(chroma.Keyword):
		return segCodeKeyword
	case t.InCategory(chroma.Name):
		if data || t == chroma.NameTag || t == chroma.NameAttribute {
			return segCodeKey
		}
		return segCode
	case t.InCategory(chroma.Punctuation), t.InCategory(chroma.Operator):
		return segCodePunct
	case t == chroma.GenericHeading, t == chroma.GenericSubheading:
		return segCodeHeading
	case t == chroma.GenericStrong:
		return segCodeStrong
	case t == chroma.GenericEmph:
		return segCodeEmph
	}
	return segCode
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
