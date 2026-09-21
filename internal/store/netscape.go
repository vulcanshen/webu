package store

import (
	"errors"
	"html"
	"io"
	"regexp"
	"strings"
)

// Import is a browser's bookmarks export, read for the Bookmarks screen's
// [I]mport: the bookmarks, each with the folder path it sat in relative to
// the file's top, and every folder the file declares — an empty one too —
// in the order met. The UI puts the lot under a folder of the user's
// naming (ui bookmarks.go).
type Import struct {
	Bookmarks []Bookmark
	Folders   []string
}

// The Netscape Bookmark File Format is HTML with a fixed shape: a <DT><H3>
// names a folder and the <DL> after it holds what is in it, a <DT><A HREF>
// is a bookmark, </DL> closes the folder. Only those four tags matter.
var (
	netscapeTag = regexp.MustCompile(`(?is)<(h3|a|dl|/dl)\b([^>]*)>`)
	hrefAttr    = regexp.MustCompile(`(?is)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	closeH3     = regexp.MustCompile(`(?i)</h3>`)
	closeA      = regexp.MustCompile(`(?i)</a>`)
	anyTag      = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespace  = regexp.MustCompile(`\s+`)
)

// ParseNetscape reads the Netscape Bookmark File Format — what Chrome,
// Firefox, Safari and Edge all export — into an Import. Attributes such as
// ADD_DATE and ICON are dropped; entities are decoded; a slash in a
// folder's name becomes a dash, since "/" separates levels in
// bookmarks.yaml. Firefox's place: entries (its smart folders) are not
// bookmarks and are skipped. A file with neither a bookmark nor a folder
// in it is refused: it is not an export.
func ParseNetscape(r io.Reader) (Import, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Import{}, err
	}
	s := string(raw)
	var imp Import
	var stack []string // one folder name per open <DL>; "" for the top
	pending := ""      // the <H3> waiting for its <DL>
	seen := map[string]bool{}
	for _, loc := range netscapeTag.FindAllStringSubmatchIndex(s, -1) {
		tag := strings.ToLower(s[loc[2]:loc[3]])
		attrs := s[loc[4]:loc[5]]
		switch tag {
		case "h3":
			pending = strings.ReplaceAll(textUntil(s[loc[1]:], closeH3), "/", "-")
		case "dl":
			stack = append(stack, pending)
			pending = ""
			if f := joinFolder(stack); f != "" && !seen[f] {
				seen[f] = true
				imp.Folders = append(imp.Folders, f)
			}
		case "/dl":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case "a":
			href := ""
			if m := hrefAttr.FindStringSubmatch(attrs); m != nil {
				href = strings.TrimSpace(html.UnescapeString(m[1] + m[2] + m[3]))
			}
			if href == "" || strings.HasPrefix(strings.ToLower(href), "place:") {
				continue
			}
			title := textUntil(s[loc[1]:], closeA)
			if title == "" {
				title = href
			}
			imp.Bookmarks = append(imp.Bookmarks, Bookmark{Title: title, URL: href, Folder: joinFolder(stack)})
		}
	}
	if len(imp.Bookmarks) == 0 && len(imp.Folders) == 0 {
		return Import{}, errors.New("no bookmarks in it; is this a browser's bookmarks export?")
	}
	return imp, nil
}

// textUntil is the text from the start of rest to the first close tag:
// tags inside dropped, entities decoded, whitespace collapsed.
func textUntil(rest string, close *regexp.Regexp) string {
	end := len(rest)
	if i := close.FindStringIndex(rest); i != nil {
		end = i[0]
	} else if i := strings.Index(rest, "<"); i >= 0 {
		end = i
	}
	t := anyTag.ReplaceAllString(rest[:end], "")
	return strings.TrimSpace(whitespace.ReplaceAllString(html.UnescapeString(t), " "))
}

// joinFolder is the path of the open folders, the nameless top left out.
func joinFolder(stack []string) string {
	var parts []string
	for _, f := range stack {
		if f != "" {
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, "/")
}
