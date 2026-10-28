// Package reader is webu's reader (docs/reader.md): a page fetched over
// net/http, its content picked out of the site around it, built into
// webu's content tree and written out as markdown — for now; webu's own
// elements later. No Chromium.
package reader

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

// Jar is the cookie jar of the reader's pages: Go's, which does the
// matching, with every cookie it is handed also written to a file, so a
// sign-in outlives webu. The file is the user's alone (0600), and no value
// goes to the App Log or the screen. A session cookie is kept too: it is
// what most sign-ins are, and signing in again at every start is what the
// file is there to spare.
type Jar struct {
	mu    sync.Mutex
	jar   *cookiejar.Jar
	path  string
	saved map[string]savedCookie
	err   error
}

// savedCookie is one cookie as the file keeps it: enough to hand it back
// to the jar as the site set it.
type savedCookie struct {
	// Host is the host it came from: the one it is for, when it named no
	// Domain.
	Host     string    `json:"host"`
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitzero"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"httpOnly,omitempty"`
}

// OpenJar reads the jar's file at path; an empty path is a jar that
// forgets at the end. A file that is not there is an empty jar; one that
// cannot be read is an error, for the MOVE ASIDE question (tdp E3).
func OpenJar(path string) (*Jar, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	j := &Jar{jar: jar, path: path, saved: map[string]savedCookie{}}
	if path == "" {
		return j, nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return j, nil
	}
	if err != nil {
		return nil, err
	}
	var list []savedCookie
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	now := time.Now()
	for _, s := range list {
		if !s.Expires.IsZero() && s.Expires.Before(now) {
			continue
		}
		j.saved[s.key()] = s
		u := &url.URL{Scheme: "https", Host: s.Host, Path: nameOr(s.Path, "/")}
		j.jar.SetCookies(u, []*http.Cookie{s.cookie()})
	}
	return j, nil
}

func (s savedCookie) key() string {
	return strings.Join([]string{nameOr(s.Domain, s.Host), s.Path, s.Name}, "\x00")
}

func (s savedCookie) cookie() *http.Cookie {
	return &http.Cookie{Name: s.Name, Value: s.Value, Domain: s.Domain, Path: s.Path,
		Expires: s.Expires, Secure: s.Secure, HttpOnly: s.HTTPOnly}
}

// SetCookies is http.CookieJar's: the jar takes them, and the file is
// written again.
func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.jar.SetCookies(u, cookies)
	now := time.Now()
	for _, c := range cookies {
		s := savedCookie{Host: u.Hostname(), Name: c.Name, Value: c.Value,
			Domain: strings.TrimPrefix(strings.ToLower(c.Domain), "."), Path: c.Path,
			Expires: c.Expires, Secure: c.Secure, HTTPOnly: c.HttpOnly}
		if s.Path == "" {
			s.Path = defaultPath(u.Path)
		}
		if c.MaxAge > 0 {
			s.Expires = now.Add(time.Duration(c.MaxAge) * time.Second)
		}
		if c.MaxAge < 0 || !s.Expires.IsZero() && s.Expires.Before(now) {
			delete(j.saved, s.key())
			continue
		}
		j.saved[s.key()] = s
	}
	j.err = j.save()
}

// Cookies is http.CookieJar's.
func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.jar.Cookies(u)
}

// Clear forgets every cookie and takes the file away: signed out of
// everything.
func (j *Jar) Clear() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}
	j.jar, j.saved = jar, map[string]savedCookie{}
	if j.path == "" {
		return nil
	}
	if err := os.Remove(j.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Err is the last time the file could not be written, once: the app says
// so, and the cookies stay in memory meanwhile.
func (j *Jar) Err() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	err := j.err
	j.err = nil
	return err
}

// save writes the file whole, atomically, readable by the user alone.
func (j *Jar) save() error {
	if j.path == "" {
		return nil
	}
	list := make([]savedCookie, 0, len(j.saved))
	for _, s := range j.saved {
		list = append(list, s)
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return err
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, j.path)
}

// defaultPath is a cookie's path when the site named none (RFC 6265 5.1.4):
// the request path up to its last slash.
func defaultPath(p string) string {
	if p == "" || p[0] != '/' {
		return "/"
	}
	i := strings.LastIndex(p, "/")
	if i == 0 {
		return "/"
	}
	return p[:i]
}

func nameOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
