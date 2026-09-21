package page

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/domstorage"
	cdplog "github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

// What the DevTools popup shows (ui.md §3.2): the tab's network requests
// and console since the last navigation, and the site's cookies and web
// storage on demand. The log is filled from CDP events on the tab's own
// goroutine and read from the UI's, so it carries a lock.

// DevLogCap is how many requests and how many console lines a tab keeps:
// a ring, so a page that never stops talking cannot grow without bound.
const DevLogCap = 500

// NetEntry is one request as the Network tab lists it.
type NetEntry struct {
	ID       network.RequestID
	Method   string
	URL      string
	Type     string
	Status   int64
	Mime     string
	Size     float64
	Started  time.Time
	Duration time.Duration
	Error    string
	Done     bool
	ReqHdr   map[string]string
	RespHdr  map[string]string
}

// ConsoleEntry is one line of the Console tab.
type ConsoleEntry struct {
	Level string // log / info / warning / error
	Text  string
	At    time.Time
	Where string // url:line when known
}

// DevLog is a tab's record. Zero value is ready.
type DevLog struct {
	mu      sync.Mutex
	net     []*NetEntry
	byID    map[network.RequestID]*NetEntry
	console []ConsoleEntry
}

// Net is a copy of the requests, oldest first.
func (l *DevLog) Net() []NetEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]NetEntry, len(l.net))
	for i, e := range l.net {
		out[i] = *e
	}
	return out
}

// Console is a copy of the console lines, oldest first.
func (l *DevLog) Console() []ConsoleEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ConsoleEntry(nil), l.console...)
}

func (l *DevLog) ClearNet() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.net, l.byID = nil, nil
}

func (l *DevLog) ClearConsole() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.console = nil
}

func (l *DevLog) addNet(e *NetEntry) {
	if l.byID == nil {
		l.byID = map[network.RequestID]*NetEntry{}
	}
	l.byID[e.ID] = e
	l.net = append(l.net, e)
	if len(l.net) > DevLogCap {
		delete(l.byID, l.net[0].ID)
		l.net = l.net[1:]
	}
}

func (l *DevLog) addConsole(e ConsoleEntry) {
	l.console = append(l.console, e)
	if len(l.console) > DevLogCap {
		l.console = l.console[1:]
	}
}

// Add appends a line from webu's side: what was typed at the console and
// what came back (Eval). Levels "input" and "result" are webu's own.
func (l *DevLog) Add(e ConsoleEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.At.IsZero() {
		e.At = time.Now()
	}
	l.addConsole(e)
}

// Eval runs an expression in the page the way Chrome's console does —
// REPL mode, promises awaited, the value brought back — and returns the
// line to show for it: the result, or the exception.
func Eval(ctx context.Context, expr string) ConsoleEntry {
	var out ConsoleEntry
	err := run(ctx, func(ctx context.Context) error {
		// Not returnByValue: that serialises the result, and `window` (or
		// anything with a cycle) then fails with "Object reference chain is
		// too long" — which is what the prompt printed for `window` until
		// 2026-09-21. The object comes back by reference with a preview,
		// and is printed the way the console prints one.
		obj, exc, err := runtime.Evaluate(expr).WithGeneratePreview(true).
			WithAwaitPromise(true).WithReplMode(true).Do(ctx)
		if err != nil {
			return err
		}
		if exc != nil {
			text := exc.Text
			if exc.Exception != nil && exc.Exception.Description != "" {
				text = exc.Exception.Description
			}
			out = ConsoleEntry{Level: "error", Text: text}
			return nil
		}
		text := resultText(obj)
		if obj != nil && obj.ObjectID != "" && obj.Type == runtime.TypeObject && obj.Subtype != runtime.SubtypeNull {
			text = objectText(ctx, obj)
		}
		out = ConsoleEntry{Level: "result", Text: text}
		return nil
	})
	if err != nil {
		return ConsoleEntry{Level: "error", Text: err.Error()}
	}
	return out
}

// objectText prints an object: a plain object or an array as its JSON
// (`{"a":1}`, `[1,2]`), which says everything; anything else — a Window, a
// document, a Location — by its class and the preview of its properties
// (`Window {window: Window, self: Window, …}`), which is what the console
// shows, since their JSON is empty or impossible.
func objectText(ctx context.Context, o *runtime.RemoteObject) string {
	if o.ClassName == "Object" || o.ClassName == "Array" {
		res, _, err := runtime.CallFunctionOn(
			"function() { try { const s = JSON.stringify(this); return s === undefined ? null : s } catch (e) { return null } }").
			WithObjectID(o.ObjectID).WithReturnByValue(true).Do(ctx)
		if err == nil && res != nil && res.Type == runtime.TypeString {
			var s string
			if json.Unmarshal(res.Value, &s) == nil {
				return s
			}
		}
	}
	return previewText(o)
}

// previewText is the preview Chromium attaches to an object: its
// description and the first properties, an ellipsis where it cut them.
func previewText(o *runtime.RemoteObject) string {
	p := o.Preview
	if p == nil {
		if o.Description != "" {
			return o.Description
		}
		return string(o.Type)
	}
	parts := make([]string, 0, len(p.Properties))
	for _, pr := range p.Properties {
		v := pr.Value
		switch {
		case pr.Type == runtime.TypeString:
			v = strconvQuote(pr.Value)
		case pr.Type == runtime.TypeFunction:
			v = "ƒ"
		case pr.Type == runtime.TypeObject && pr.ValuePreview != nil && pr.ValuePreview.Description != "":
			v = pr.ValuePreview.Description
		case pr.Type == runtime.TypeObject && v == "":
			v = "Object"
		}
		parts = append(parts, pr.Name+": "+v)
	}
	inner := strings.Join(parts, ", ")
	if p.Overflow {
		inner += ", …"
	}
	switch {
	case o.Subtype == runtime.SubtypeArray:
		return p.Description + " [" + inner + "]"
	case p.Description == "Object":
		return "{" + inner + "}"
	case inner == "":
		return p.Description // a Date, a RegExp: the description is the value
	}
	return p.Description + " {" + inner + "}"
}

// resultText is a value the way the console prints one: strings quoted,
// numbers and booleans as written, the rest by value or description.
func resultText(o *runtime.RemoteObject) string {
	if o == nil {
		return "undefined"
	}
	switch o.Type {
	case runtime.TypeUndefined:
		return "undefined"
	case runtime.TypeString:
		var s string
		if err := json.Unmarshal(o.Value, &s); err == nil {
			return strconvQuote(s)
		}
	}
	if len(o.Value) > 0 {
		return string(o.Value)
	}
	if o.UnserializableValue != "" {
		return string(o.UnserializableValue)
	}
	if o.Description != "" {
		return o.Description
	}
	return string(o.Type)
}

// strconvQuote is the console's single-quoted string, escapes kept simple.
func strconvQuote(s string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(s) + "'"
}

// Observe records the tab's traffic and console into l. Registered on the
// tab before it is attached, like the other listeners; the domains that
// produce the events are enabled by Prepare.
//
// Both logs empty on a main-frame navigation, the way Chrome's own panels
// do by default; a "preserve log" switch is v2 (ui.md §3.2).
func Observe(ctx context.Context, l *DevLog) {
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *cdppage.EventFrameNavigated:
			if e.Frame.ParentID == "" {
				l.ClearNet()
				l.ClearConsole()
			}
		case *network.EventRequestWillBeSent:
			l.mu.Lock()
			if e.RedirectResponse != nil {
				// The same request id continues after a redirect: close the
				// hop that redirected, then log the new one.
				if prev := l.byID[e.RequestID]; prev != nil {
					prev.Status, prev.Done = e.RedirectResponse.Status, true
					prev.Duration = time.Since(prev.Started)
					delete(l.byID, e.RequestID)
				}
			}
			entry := &NetEntry{ID: e.RequestID, Started: time.Now(), Type: string(e.Type)}
			if e.Request != nil {
				entry.Method, entry.URL = e.Request.Method, e.Request.URL
				entry.ReqHdr = headers(e.Request.Headers)
			}
			l.addNet(entry)
			l.mu.Unlock()
		case *network.EventResponseReceived:
			l.mu.Lock()
			if x := l.byID[e.RequestID]; x != nil && e.Response != nil {
				x.Status, x.Mime, x.Type = e.Response.Status, e.Response.MimeType, string(e.Type)
				x.RespHdr = headers(e.Response.Headers)
				if len(x.ReqHdr) == 0 {
					x.ReqHdr = headers(e.Response.RequestHeaders)
				}
			}
			l.mu.Unlock()
		case *network.EventLoadingFinished:
			l.mu.Lock()
			if x := l.byID[e.RequestID]; x != nil {
				x.Size, x.Done = e.EncodedDataLength, true
				x.Duration = time.Since(x.Started)
			}
			l.mu.Unlock()
		case *network.EventLoadingFailed:
			l.mu.Lock()
			if x := l.byID[e.RequestID]; x != nil {
				x.Error, x.Done = e.ErrorText, true
				if x.Error == "" && e.Canceled {
					x.Error = "canceled"
				}
				x.Duration = time.Since(x.Started)
			}
			l.mu.Unlock()
		case *runtime.EventConsoleAPICalled:
			parts := make([]string, 0, len(e.Args))
			for _, a := range e.Args {
				parts = append(parts, remoteText(a))
			}
			entry := ConsoleEntry{Level: consoleLevel(string(e.Type)), Text: strings.Join(parts, " "), At: time.Now()}
			if e.StackTrace != nil && len(e.StackTrace.CallFrames) > 0 {
				f := e.StackTrace.CallFrames[0]
				entry.Where = shortURL(f.URL) + ":" + itoa(int(f.LineNumber)+1)
			}
			l.mu.Lock()
			l.addConsole(entry)
			l.mu.Unlock()
		case *runtime.EventExceptionThrown:
			d := e.ExceptionDetails
			if d == nil {
				return
			}
			text := d.Text
			if d.Exception != nil && d.Exception.Description != "" {
				text = d.Exception.Description
			}
			entry := ConsoleEntry{Level: "error", Text: text, At: time.Now()}
			if d.URL != "" {
				entry.Where = shortURL(d.URL) + ":" + itoa(int(d.LineNumber)+1)
			}
			l.mu.Lock()
			l.addConsole(entry)
			l.mu.Unlock()
		case *cdplog.EventEntryAdded:
			if e.Entry == nil {
				return
			}
			entry := ConsoleEntry{Level: string(e.Entry.Level), Text: e.Entry.Text, At: time.Now()}
			if e.Entry.URL != "" {
				entry.Where = shortURL(e.Entry.URL)
				if e.Entry.LineNumber > 0 {
					entry.Where += ":" + itoa(int(e.Entry.LineNumber))
				}
			}
			l.mu.Lock()
			l.addConsole(entry)
			l.mu.Unlock()
		}
	})
}

// enableDevDomains turns on the event streams Observe listens to. Part of
// Prepare, so a request the page makes on its first document is seen.
func enableDevDomains(ctx context.Context) error {
	if err := network.Enable().Do(ctx); err != nil {
		return err
	}
	if err := runtime.Enable().Do(ctx); err != nil {
		return err
	}
	return cdplog.Enable().Do(ctx)
}

func headers(h network.Headers) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		switch x := v.(type) {
		case string:
			out[k] = x
		default:
			b, _ := json.Marshal(x)
			out[k] = string(b)
		}
	}
	return out
}

func consoleLevel(apiType string) string {
	switch apiType {
	case "error", "assert":
		return "error"
	case "warning":
		return "warning"
	case "info", "log", "debug", "trace", "dir", "dirxml", "table", "count", "timeEnd":
		return "log"
	}
	return apiType
}

// remoteText is a console argument as text: a string unquoted, a number
// or boolean as written, anything else by its description.
func remoteText(o *runtime.RemoteObject) string {
	if o == nil {
		return ""
	}
	if o.Type == runtime.TypeString {
		var s string
		if err := json.Unmarshal(o.Value, &s); err == nil {
			return s
		}
	}
	if len(o.Value) > 0 {
		return string(o.Value)
	}
	if o.Description != "" {
		return o.Description
	}
	if o.UnserializableValue != "" {
		return string(o.UnserializableValue)
	}
	return string(o.Type)
}

func shortURL(u string) string {
	if i := strings.LastIndexByte(u, '/'); i >= 0 && i < len(u)-1 {
		return u[i+1:]
	}
	return u
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ResponseBody is a finished request's body, as text (base64 content is
// left as is and said so).
func ResponseBody(ctx context.Context, id network.RequestID) (string, error) {
	var body string
	err := run(ctx, func(ctx context.Context) error {
		b, err := network.GetResponseBody(id).Do(ctx)
		if err != nil {
			return err
		}
		// cdproto has already decoded a base64 body; binary is shown as is,
		// which for an image is noise the viewer clips.
		body = string(b)
		return nil
	})
	return body, err
}

// ------------------------------------------------------------- storage

// StorageData is the Storage tab: the site's cookies and its local and
// session storage, fetched fresh each time the tab is shown.
type StorageData struct {
	Origin  string
	Cookies []*network.Cookie
	Local   [][2]string
	Session [][2]string
}

// Origin is the scheme://host[:port] of u, or "" for a URL without one.
func Origin(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Scheme == "" || p.Host == "" {
		return ""
	}
	return p.Scheme + "://" + p.Host
}

// StorageFor reads the site's cookies and web storage. A file:// page has
// no origin Chromium will answer for; it comes back empty rather than as an
// error.
func StorageFor(ctx context.Context, pageURL string) (StorageData, error) {
	d := StorageData{Origin: Origin(pageURL)}
	err := run(ctx, func(ctx context.Context) error {
		cookies, err := network.GetCookies().WithURLs([]string{pageURL}).Do(ctx)
		if err != nil {
			return err
		}
		sort.Slice(cookies, func(i, j int) bool { return cookies[i].Name < cookies[j].Name })
		d.Cookies = cookies
		if d.Origin == "" {
			return nil
		}
		for _, local := range []bool{true, false} {
			items, err := domstorage.GetDOMStorageItems(&domstorage.StorageID{SecurityOrigin: d.Origin, IsLocalStorage: local}).Do(ctx)
			if err != nil {
				continue // an origin with no storage area answers with an error
			}
			var kv [][2]string
			for _, it := range items {
				if len(it) == 2 {
					kv = append(kv, [2]string{it[0], it[1]})
				}
			}
			sort.Slice(kv, func(i, j int) bool { return kv[i][0] < kv[j][0] })
			if local {
				d.Local = kv
			} else {
				d.Session = kv
			}
		}
		return nil
	})
	return d, err
}

// DeleteCookie removes exactly this cookie.
func DeleteCookie(ctx context.Context, c *network.Cookie) error {
	return run(ctx, func(ctx context.Context) error {
		p := network.DeleteCookies(c.Name).WithDomain(c.Domain).WithPath(c.Path)
		if c.PartitionKey != nil {
			p = p.WithPartitionKey(c.PartitionKey)
		}
		return p.Do(ctx)
	})
}

// RemoveStorageItem drops one key from the origin's local or session storage.
func RemoveStorageItem(ctx context.Context, origin string, local bool, key string) error {
	return run(ctx, func(ctx context.Context) error {
		return domstorage.RemoveDOMStorageItem(&domstorage.StorageID{SecurityOrigin: origin, IsLocalStorage: local}, key).Do(ctx)
	})
}

// ClearSiteData is the Storage tab's Clear site data (function.md §8):
// everything Chromium keeps for the origin, cookies included.
func ClearSiteData(ctx context.Context, origin string) error {
	return run(ctx, func(ctx context.Context) error {
		return storage.ClearDataForOrigin(origin, "all").Do(ctx)
	})
}
