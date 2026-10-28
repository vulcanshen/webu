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
// waits for ever (tdp F6). Each is the lib's popup for it, whose cancel
// tells the page no: a confirm (alert, confirm, beforeunload), a text
// input (prompt), a form (sign-in).

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
	answer := func(accept bool, v string) tea.Cmd { cmd := m.answerDialog(accept, v); m.sync(); return cmd }
	cancel := func() tea.Cmd { return answer(false, "") }
	if msg.kind == cdppage.DialogTypePrompt {
		// The answer the page offers is offered in grey: Enter on it as it
		// is answers with it (Unchanged), an emptied one answers empty. The
		// page's whole message is the prompt, its line breaks too: the lib
		// folds it, the popup growing with it (tdp input.2).
		return m.a.Text(terminu.TextInput{Title: "The page asks", Glyph: glyphInfo, Name: "The answer",
			Prompt: text, Verb: "answer", Old: msg.defaultPrompt,
			Done:      func(v string) tea.Cmd { return answer(true, v) },
			Unchanged: func() tea.Cmd { return answer(true, msg.defaultPrompt) },
			OnCancel:  cancel})
	}
	ask := terminu.Ask{Title: "The page says", Details: []string{text}, Question: "Dismiss the page's message?", Verb: "dismiss",
		OnCancel: cancel}
	switch msg.kind {
	case cdppage.DialogTypeConfirm:
		ask.Title, ask.Question, ask.Verb = "The page asks", "Answer yes?", "yes"
	case cdppage.DialogTypeBeforeunload:
		ask.Title, ask.Details, ask.Question, ask.Verb = "Leave this page", []string{"The page says it has unsaved changes."}, "Leave it anyway?", "leave"
	}
	return m.a.Confirm(ask, func() tea.Cmd { return answer(true, "") })
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

// askAuth is an HTTP challenge: a form of a name and a password, sent
// together; the request waits until they are, and a form closed without
// signing in sends the page its 401 (Form.OnCancel).
func (m *AppModel) askAuth(msg authMsg) tea.Cmd {
	where := msg.origin
	if msg.realm != "" {
		where += " — " + msg.realm
	}
	name, pass := "", ""
	nameField := &terminu.FormField{Label: "Name", Value: func() string { return name }, Empty: "not set",
		Edit: func() terminu.Input {
			return terminu.TextInput{Title: "Name", Glyph: glyphPencil, Name: "The name", Prompt: "Your name on this site", Verb: "set", Old: name,
				Done: func(v string) tea.Cmd { name = v; return nil }}
		}}
	passField := &terminu.FormField{Label: "Password", Empty: "not set",
		// Set, eight dots, whatever its length (tdp input.5).
		Value: func() string {
			if pass == "" {
				return ""
			}
			return strings.Repeat("•", 8)
		},
		Edit: func() terminu.Input {
			return terminu.PasswordInput{Title: "Password", Glyph: glyphPencil, Prompt: "Your password on this site", Verb: "set", Set: pass != "",
				Done: func(v string, clear bool) tea.Cmd {
					pass = v
					if clear {
						pass = ""
					}
					return nil
				}}
		}}
	return m.a.Form(&terminu.Form{Title: "Sign in", Subject: where, Glyph: glyphPencil, Name: "the sign-in",
		Fields: []*terminu.FormField{nameField, passField}, Button: "Sign in",
		Submit: func() (tea.Cmd, error) {
			a := m.auth
			m.auth = nil
			if a == nil {
				return nil, nil
			}
			_, t := m.tabByID(a.tabID)
			if t == nil {
				return nil, nil
			}
			user, password := name, pass
			return t.unblock(func(ctx context.Context) error { return page.Auth(ctx, a.id, user, password) }), nil
		},
		OnCancel: m.cancelAuth})
}

// cancelAuth is a sign-in closed without signing in: the page gets its 401.
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
