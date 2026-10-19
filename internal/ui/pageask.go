package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cdppage "github.com/chromedp/cdproto/page"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/page"
)

// The page's own questions (function.md §5): alert, confirm, beforeunload,
// prompt, and an HTTP sign-in. The page is stalled until it has an answer,
// and cancelling is an answer too — the page must be told "no", or it
// waits for ever. Alert, confirm and beforeunload are the lib's confirm,
// whose OnCancel tells the page no.
//
// A prompt and a sign-in are popups of webu's own (App.Popup) that look
// like a confirm: what the page said, the answer it will get, the question
// last; Enter answers, Esc cancels (dev-remarks, 偏離 tdp).
// TODO(lib WB17): a prompt answers with the lib's text input once Enter on
// the value it opened with — an empty default answer — is an answer the
// app hears; now nothing is called and the page would wait for ever.
// TODO(lib WB18): a sign-in is the lib's form once a form says when Esc
// closed it.

type askKind uint8

const (
	askPrompt askKind = iota // prompt: an answer to give, e edits it
	askAuth                  // sign in: a name and a password
)

// pageAsk is one of the page's questions on screen.
type pageAsk struct {
	m        *AppModel
	kind     askKind
	details  []string
	question string
	verb     string
	answer   string // the prompt's answer
	name     string // the sign-in's
	pass     string
	accept   func() tea.Cmd
	cancel   func() tea.Cmd
	top      int  // the first detail row in sight, when they do not all fit
	over     bool // they do not: j|k is in the hint
}

// valueW is how wide the label column of the values is.
const valueW = 10

func (p *pageAsk) Size(sw, sh int) (int, int) {
	w := dispW(p.question)
	for _, d := range p.details {
		w = max(w, dispW(d))
	}
	switch p.kind {
	case askPrompt:
		w = max(w, valueW+dispW(p.answer))
	case askAuth:
		w = max(w, valueW+dispW(p.name))
	}
	// A cell of margin either side; no wider than the screen allows.
	w = min(w+2, max(min(sw-2, 120)-2, 10))
	details, tail := p.parts(w)
	return w, min(len(details)+len(tail), max(sh-6, 3))
}

// parts are what the popup holds, wrapped to w less its margins: what the
// page said, which scrolls; then the values that will be sent and the
// question, which stay.
func (p *pageAsk) parts(w int) (details, tail []string) {
	inner := max(w-2, 1)
	for _, d := range p.details {
		for _, l := range terminu.Wrap(d, inner) {
			details = append(details, terminu.Fg(terminu.Overlay0).Render(l))
		}
	}
	value := func(label, v string) string {
		return terminu.Fg(terminu.Text).Render(padRight(label, valueW)) + terminu.Fg(terminu.Lavender).Render(truncate(v, max(1, inner-valueW)))
	}
	switch p.kind {
	case askPrompt:
		tail = append(tail, "", value("ANSWER", p.answer))
	case askAuth:
		tail = append(tail, "", value("NAME", p.name), value("PASSWORD", strings.Repeat("•", len([]rune(p.pass)))))
	}
	if len(details)+len(tail) > 0 {
		tail = append(tail, "")
	}
	for _, l := range terminu.Wrap(p.question, inner) {
		tail = append(tail, terminu.Fg(terminu.Text).Render(l))
	}
	return details, tail
}

func (p *pageAsk) Draw(w, h int, _ terminu.Color) []string {
	details, tail := p.parts(w)
	room := max(h-len(tail), 0)
	p.top = min(max(p.top, 0), max(len(details)-room, 0))
	p.over = len(details) > room
	var rows []string
	for i := p.top; i < len(details) && len(rows) < room; i++ {
		rows = append(rows, details[i])
	}
	rows = append(rows, tail...)
	out := make([]string, 0, h)
	for _, r := range rows {
		out = append(out, " "+terminu.Pad(r, w-2)+" ")
	}
	return terminu.Pad2(out, w, h)
}

func (p *pageAsk) Key(key string, _ tea.KeyMsg) (bool, tea.Cmd) {
	switch key {
	case "Enter":
		return true, tea.Batch(p.m.a.ClosePopup(p), p.accept())
	case "Esc":
		return true, tea.Batch(p.m.a.ClosePopup(p), p.cancel())
	case "j", "↓":
		p.top++
		return true, nil
	case "k", "↑":
		p.top--
		return true, nil
	case "e":
		if p.kind == askPrompt {
			return true, p.m.a.Text(terminu.TextInput{Title: "Answer", Glyph: glyphPencil, Name: "The answer",
				Prompt: "What to answer the page", Verb: "set", Old: p.answer,
				Done: func(v string) tea.Cmd { p.answer = v; return nil }})
		}
	case "n":
		if p.kind == askAuth {
			return true, p.m.a.Text(terminu.TextInput{Title: "Sign in", Glyph: glyphPencil, Name: "The name",
				Prompt: "Your name on this site", Verb: "set", Old: p.name,
				Done: func(v string) tea.Cmd { p.name = v; return nil }})
		}
	case "p":
		if p.kind == askAuth {
			return true, p.m.a.Password(terminu.PasswordInput{Title: "Sign in", Glyph: glyphPencil,
				Prompt: "Your password on this site", Verb: "set", Set: p.pass != "",
				Done: func(v string, clear bool) tea.Cmd {
					p.pass = v
					if clear {
						p.pass = ""
					}
					return nil
				}})
		}
	}
	return false, nil
}

func (p *pageAsk) Hint() []terminu.KeyHint {
	h := []terminu.KeyHint{{Key: "Enter", Desc: p.verb}}
	switch p.kind {
	case askPrompt:
		h = append(h, terminu.KeyHint{Key: "e", Desc: "edit the answer"})
	case askAuth:
		h = append(h, terminu.KeyHint{Key: "n", Desc: "name"}, terminu.KeyHint{Key: "p", Desc: "password"})
	}
	if p.over {
		h = append(h, terminu.KeyHint{Key: "j|k", Desc: "scroll"})
	}
	return append(h, terminu.KeyHint{Key: "Esc", Desc: "cancel"})
}

func (p *pageAsk) Keys() []terminu.KeySection {
	rows := []terminu.KeyRow{{Key: "Enter", Desc: p.verb + ": send the page this answer"}}
	switch p.kind {
	case askPrompt:
		rows = append(rows, terminu.KeyRow{Key: "e", Desc: "edit the answer"})
	case askAuth:
		rows = append(rows, terminu.KeyRow{Key: "n", Desc: "edit the name"}, terminu.KeyRow{Key: "p", Desc: "edit the password"})
	}
	rows = append(rows, terminu.KeyRow{Key: "j|k", Desc: "scroll what the page said, when it is long"},
		terminu.KeyRow{Key: "Esc", Desc: "cancel: the page is told no"})
	return []terminu.KeySection{{Title: "the page asks", Rows: rows}}
}

// askDialog puts a page's question on screen (function.md §5). The dialog
// is remembered so that Esc, too, sends an answer — a page left waiting on
// a dialog nobody answers is a page that has stopped.
func (m *AppModel) askDialog(msg dialogMsg) tea.Cmd {
	_, t := m.tabByID(msg.tabID)
	if t == nil {
		return m.answerDialogOn(msg.tabID, false, "")
	}
	m.dialog = &msg
	// The click that raised this will not return until the dialog is
	// answered, so the settle behind it never runs: the tab stops saying it
	// is busy here instead (2026-09-22).
	t.loading = false
	text := msg.message
	if strings.TrimSpace(text) == "" {
		text = "(no message)"
	}
	cancel := func() tea.Cmd { cmd := m.answerDialog(false, ""); m.sync(); return cmd }
	if msg.kind == cdppage.DialogTypePrompt {
		p := &pageAsk{m: m, kind: askPrompt, details: []string{text}, answer: msg.defaultPrompt,
			question: "Send this answer?", verb: "answer", cancel: cancel}
		p.accept = func() tea.Cmd { cmd := m.answerDialog(true, p.answer); m.sync(); return cmd }
		return m.a.Popup(terminu.Popup{Title: "The page asks", Glyph: glyphInfo, Body: p})
	}
	ask := terminu.Ask{Title: "The page says", Details: []string{text}, Question: "Dismiss the page's message?", Verb: "dismiss",
		OnCancel: cancel}
	switch msg.kind {
	case cdppage.DialogTypeConfirm:
		ask.Title, ask.Question, ask.Verb = "The page asks", "Answer yes?", "yes"
	case cdppage.DialogTypeBeforeunload:
		ask.Title, ask.Details, ask.Question, ask.Verb = "Leave this page", []string{"The page says it has unsaved changes."}, "Leave it anyway?", "leave"
	}
	return m.a.Confirm(ask, func() tea.Cmd { cmd := m.answerDialog(true, ""); m.sync(); return cmd })
}

// answerDialog sends the pending dialog its answer and forgets it. An
// alert has only one answer, so a cancel on it still accepts.
func (m *AppModel) answerDialog(accept bool, text string) tea.Cmd {
	d := m.dialog
	if d == nil {
		return nil
	}
	m.dialog = nil
	if d.kind == cdppage.DialogTypeAlert {
		accept = true
	}
	return m.answerDialogOn(d.tabID, accept, text)
}

func (m *AppModel) answerDialogOn(tabID int, accept bool, text string) tea.Cmd {
	_, t := m.tabByID(tabID)
	if t == nil {
		return nil
	}
	return t.unblock(func(ctx context.Context) error { return page.HandleDialog(ctx, accept, text) })
}

// askAuth is an HTTP challenge: a name and a password, sent together; the
// request waits until they are, and Esc sends the page its 401.
func (m *AppModel) askAuth(msg authMsg) tea.Cmd {
	where := msg.origin
	if msg.realm != "" {
		where += " — " + msg.realm
	}
	p := &pageAsk{m: m, kind: askAuth, details: []string{where}, question: "Sign in with these?", verb: "sign in"}
	p.accept = func() tea.Cmd {
		a := m.auth
		m.auth = nil
		if a == nil {
			return nil
		}
		_, t := m.tabByID(a.tabID)
		if t == nil {
			return nil
		}
		user, pass := p.name, p.pass
		return t.unblock(func(ctx context.Context) error { return page.Auth(ctx, a.id, user, pass) })
	}
	p.cancel = m.cancelAuth
	return m.a.Popup(terminu.Popup{Title: "Sign in", Glyph: glyphPencil, Body: p})
}

// cancelAuth is Esc on a challenge: the page gets its 401.
func (m *AppModel) cancelAuth() tea.Cmd {
	a := m.auth
	m.auth = nil
	if a == nil {
		return nil
	}
	_, t := m.tabByID(a.tabID)
	if t == nil {
		return nil
	}
	return t.unblock(func(ctx context.Context) error { return page.CancelAuth(ctx, a.id) })
}
