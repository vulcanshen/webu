package reader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// maxPage is the most of a page read into memory: past it the page is cut
// there, which is more than any page meant to be read holds.
const maxPage = 16 << 20

// Client gets the reader's pages: Go's own HTTP, the jar's cookies, a
// browser's User-Agent with webu's name after it.
type Client struct {
	Jar  *Jar
	UA   string
	http *http.Client
}

// NewClient is a client on jar.
func NewClient(jar *Jar, ua string) *Client {
	return &Client{Jar: jar, UA: ua, http: &http.Client{Jar: jar, Timeout: 60 * time.Second}}
}

// Response is what came back: where from, after its redirects; what it is;
// a page's body, read whole; or, for anything else, a download left open
// for Save.
type Response struct {
	URL    *url.URL
	Status int
	// StatusText is "404 Not Found" and the like.
	StatusText string
	// Type is the media type, lower-cased, without its parameters;
	// ContentType is the header whole, which the charset is read from.
	Type, ContentType string
	Body              []byte
	// Filename is what the server named a download (Content-Disposition),
	// else the last part of its address; Size is its length when known.
	Filename string
	Size     int64

	download io.ReadCloser
}

// Get fetches rawURL.
func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Do sends req as the reader does every request.
func (c *Client) Do(req *http.Request) (*Response, error) {
	req.Header.Set("User-Agent", c.UA)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	}
	// No Accept-Encoding: Go asks for gzip itself and undoes it, and
	// brotli, which it cannot read, a server is then not offered.
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	r := &Response{URL: resp.Request.URL, Status: resp.StatusCode, StatusText: resp.Status,
		ContentType: resp.Header.Get("Content-Type"), Size: resp.ContentLength}
	r.Type, _, _ = mime.ParseMediaType(r.ContentType)
	r.Type = strings.ToLower(r.Type)
	r.Filename = filename(resp)
	if !r.IsPage() {
		r.download = resp.Body
		return r, nil
	}
	defer resp.Body.Close()
	r.Body, err = io.ReadAll(io.LimitReader(resp.Body, maxPage))
	return r, err
}

// IsPage reports whether this is something to read rather than to save: an
// HTML page, or text, which reads as itself. A server that names no type
// is taken at its word that it is a page, and so is an error, whatever it
// came as: what went wrong is to be read, not saved.
func (r *Response) IsPage() bool {
	switch {
	case r.Status >= 400, r.Type == "", r.Type == "text/html", r.Type == "application/xhtml+xml",
		strings.HasPrefix(r.Type, "text/"), r.Type == "application/json", strings.HasSuffix(r.Type, "+json"),
		r.Type == "application/xml", strings.HasSuffix(r.Type, "+xml"):
		return true
	}
	return false
}

// Close lets go of a download that will not be saved.
func (r *Response) Close() {
	if r.download != nil {
		r.download.Close()
	}
}

// Save writes a download into dir under its own name, a number added when
// that is taken, saying how far along it is to progress (bytes so far).
// It returns where the file went.
func (r *Response) Save(dir string, progress func(int64)) (string, error) {
	if r.download == nil {
		return "", errors.New("nothing to save")
	}
	defer r.download.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	f, p, err := create(dir, r.Filename)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, &counter{r: r.download, progress: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(p)
		return "", err
	}
	return p, nil
}

// create opens a new file in dir named name, or "name (2).ext" and on when
// that is taken: never over one already there.
func create(dir, name string) (*os.File, string, error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p := filepath.Join(dir, name)
		if i > 1 {
			p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		}
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return f, p, err
	}
}

// filename is what a download is saved as: the server's word for it, else
// the last part of its address, else "download".
func filename(resp *http.Response) string {
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		if n := filepath.Base(params["filename"]); n != "." && n != "/" && n != "" {
			return n
		}
	}
	if n := path.Base(resp.Request.URL.Path); n != "." && n != "/" && n != "" {
		if u, err := url.PathUnescape(n); err == nil {
			return filepath.Base(u)
		}
		return n
	}
	return "download"
}

type counter struct {
	r        io.Reader
	n        int64
	progress func(int64)
	last     time.Time
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	// Not every read: a progress line ten times a second is plenty.
	if c.progress != nil && (time.Since(c.last) > 100*time.Millisecond || err == io.EOF) {
		c.last = time.Now()
		c.progress(c.n)
	}
	return n, err
}
