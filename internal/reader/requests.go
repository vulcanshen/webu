package reader

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// Exchange is one request webu made and what came back: a row of the
// page's Requests popup, what is left of DevTools' Network with no
// Chromium (docs/reader.md).
type Exchange struct {
	Method, URL string
	Request     http.Header
	// Status is "301 Moved Permanently"; "" when no answer came, Err
	// saying why.
	Status   string
	Response http.Header
	// Took is the time until the answer's headers came; Size the bytes of
	// its body read, after Go has undone a gzip.
	Took time.Duration
	Size int64
	Err  string
}

// Requests is what a load asked for, in order: the page, each redirect on
// the way to it, a form's submit.
type Requests struct {
	mu   sync.Mutex
	list []*Exchange
}

type requestsKey struct{}

// Record is ctx with every request made under it kept in r.
func Record(ctx context.Context, r *Requests) context.Context {
	return context.WithValue(ctx, requestsKey{}, r)
}

// List is what has been asked so far.
func (r *Requests) List() []Exchange {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Exchange, len(r.list))
	for i, e := range r.list {
		out[i] = *e
	}
	return out
}

// recorder is the client's transport: each request whose context carries
// Requests is kept there. A redirect is a request of its own, so each one
// is.
type recorder struct{ http.RoundTripper }

func (t recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r, _ := req.Context().Value(requestsKey{}).(*Requests)
	if r == nil {
		return t.RoundTripper.RoundTrip(req)
	}
	e := &Exchange{Method: req.Method, URL: req.URL.String(), Request: req.Header.Clone()}
	r.mu.Lock()
	r.list = append(r.list, e)
	r.mu.Unlock()
	start := time.Now()
	resp, err := t.RoundTripper.RoundTrip(req)
	r.mu.Lock()
	defer r.mu.Unlock()
	e.Took = time.Since(start)
	if err != nil {
		e.Err = err.Error()
		return nil, err
	}
	e.Status, e.Response = resp.Status, resp.Header.Clone()
	resp.Body = &counted{ReadCloser: resp.Body, r: r, e: e}
	return resp, nil
}

// counted is a response body that counts what is read of it.
type counted struct {
	io.ReadCloser
	r *Requests
	e *Exchange
}

func (c *counted) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.r.mu.Lock()
	c.e.Size += int64(n)
	c.r.mu.Unlock()
	return n, err
}
