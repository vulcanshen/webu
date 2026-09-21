package store

import (
	"strings"
	"testing"
)

// chromeExport is the shape Chrome writes: the toolbar folder first, then
// Other bookmarks; ICON data on every entry; entities in text and in the
// URL; a nested folder with a slash in its name; an empty folder; an
// entry with no text; and, from Firefox, a place: entry that is not a
// bookmark.
const chromeExport = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<!-- This is an automatically generated file.
     It will be read and overwritten.
     DO NOT EDIT! -->
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="1700000000" LAST_MODIFIED="1700000001" PERSONAL_TOOLBAR_FOLDER="true">Bookmarks bar</H3>
    <DL><p>
        <DT><A HREF="https://go.dev/" ADD_DATE="1700000000" ICON="data:image/png;base64,iVBORw0KGgo=">The Go Programming Language</A>
        <DT><H3 ADD_DATE="1700000000">dev/tools</H3>
        <DL><p>
            <DT><A HREF="https://example.com/?a=1&amp;b=2">Tom &amp; Jerry&#39;s</A>
            <DT><A HREF="https://example.com/notitle"></A>
        </DL><p>
    </DL><p>
    <DT><H3 ADD_DATE="1700000000">Other bookmarks</H3>
    <DL><p>
        <DT><H3>empty</H3>
        <DL><p>
        </DL><p>
        <DT><A HREF="place:sort=8&amp;maxResults=10">Recent tags</A>
        <DT><A HREF="https://news.ycombinator.com/">Hacker
            News</A>
    </DL><p>
</DL><p>
`

func TestParseNetscape(t *testing.T) {
	imp, err := ParseNetscape(strings.NewReader(chromeExport))
	if err != nil {
		t.Fatal(err)
	}
	wantFolders := []string{"Bookmarks bar", "Bookmarks bar/dev-tools", "Other bookmarks", "Other bookmarks/empty"}
	if got := strings.Join(imp.Folders, "|"); got != strings.Join(wantFolders, "|") {
		t.Errorf("folders %q", imp.Folders)
	}
	want := []Bookmark{
		{Title: "The Go Programming Language", URL: "https://go.dev/", Folder: "Bookmarks bar"},
		{Title: "Tom & Jerry's", URL: "https://example.com/?a=1&b=2", Folder: "Bookmarks bar/dev-tools"},
		{Title: "https://example.com/notitle", URL: "https://example.com/notitle", Folder: "Bookmarks bar/dev-tools"},
		{Title: "Hacker News", URL: "https://news.ycombinator.com/", Folder: "Other bookmarks"},
	}
	if len(imp.Bookmarks) != len(want) {
		t.Fatalf("got %d bookmarks: %+v", len(imp.Bookmarks), imp.Bookmarks)
	}
	for i, b := range want {
		if imp.Bookmarks[i] != b {
			t.Errorf("bookmark %d: got %+v, want %+v", i, imp.Bookmarks[i], b)
		}
	}

	// Anything without a bookmark or a folder in it is not an export.
	if _, err := ParseNetscape(strings.NewReader("<html><body><p>hello</p></body></html>")); err == nil {
		t.Error("a page without bookmarks should be refused")
	}
}
