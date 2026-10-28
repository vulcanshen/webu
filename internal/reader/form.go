package reader

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/vulcanshen/webu/internal/ir"
)

// Form is a <form> of the page: where it goes, how, and its fields, the
// hidden ones too (docs/reader.md: GET forms and plain sign-in forms).
type Form struct {
	Name    string
	Action  *url.URL
	Method  string // GET or POST
	Enctype string
	Fields  []*Field
}

// Field is one field of a form, or one standing alone. What the user typed
// or chose is kept here, for the request, and shown on its nodes — the
// same field in the content and in the whole page.
type Field struct {
	Form *Form // nil for a field in no form: it can be filled, not sent
	// Nodes are where it is drawn: one in the content, one in the whole
	// page (either may be missing).
	Nodes []*ir.Node
	Name  string
	// Type is the input's type, lower-cased (text, password, hidden,
	// checkbox, radio, submit, image, reset, button, file, …), or
	// "textarea", "select".
	Type     string
	Value    string
	Checked  bool
	Disabled bool
	Options  []Option // a select's
	// Label is what the field is called: its <label>, aria-label,
	// placeholder, title or name, the first there is. Hint is what an
	// empty box shows: the placeholder, or a label not on the page.
	Label, Hint string
}

// Option is one option of a select.
type Option struct {
	Label, Value string
	Selected     bool
}

// Text reports whether the field takes typing.
func (f *Field) Text() bool {
	switch f.Type {
	case "hidden", "checkbox", "radio", "submit", "image", "reset", "button", "file", "select":
		return false
	}
	return true
}

// Submits reports whether pressing it sends its form.
func (f *Field) Submits() bool {
	return f.Form != nil && (f.Type == "submit" || f.Type == "image")
}

// Set is what was typed into a text field.
func (f *Field) Set(v string) {
	f.Value = v
	for _, n := range f.Nodes {
		n.Value = v
	}
}

// Toggle flips a checkbox, or picks a radio: the others of its group, in
// its form, let go.
func (f *Field) Toggle() {
	if f.Type == "radio" {
		if f.Form != nil {
			for _, o := range f.Form.Fields {
				if o != f && o.Type == "radio" && o.Name == f.Name {
					o.check(false)
				}
			}
		}
		f.check(true)
		return
	}
	f.check(!f.Checked)
}

func (f *Field) check(on bool) {
	f.Checked = on
	for _, n := range f.Nodes {
		n.Checked = ir.Off
		if on {
			n.Checked = ir.On
		}
	}
}

// Choose picks option i of a select.
func (f *Field) Choose(i int) {
	if i < 0 || i >= len(f.Options) {
		return
	}
	for j := range f.Options {
		f.Options[j].Selected = j == i
	}
	f.Value = f.Options[i].Value
	for _, n := range f.Nodes {
		n.Value = f.Options[i].Label
		for j, o := range n.Children {
			o.Selected = j == i
		}
	}
}

// chosen is a select's option that would be sent: the selected one, else
// the first.
func (f *Field) chosen() (Option, bool) {
	for _, o := range f.Options {
		if o.Selected {
			return o, true
		}
	}
	if len(f.Options) > 0 {
		return f.Options[0], true
	}
	return Option{}, false
}

// Values are what the form sends, in its order, with by the button that
// sent it (nil for none): every named field that is not disabled, a box
// only when ticked, a button only when it is the one pressed, no file.
func (fm *Form) Values(by *Field) url.Values {
	v := url.Values{}
	for _, f := range fm.Fields {
		if f.Name == "" || f.Disabled {
			continue
		}
		switch f.Type {
		case "checkbox", "radio":
			if f.Checked {
				v.Add(f.Name, nameOr(f.Value, "on"))
			}
		case "submit", "image", "button":
			if f == by {
				v.Add(f.Name, f.Value)
			}
		case "reset", "file":
		case "select":
			if o, ok := f.chosen(); ok {
				v.Add(f.Name, o.Value)
			}
		default:
			v.Add(f.Name, f.Value)
		}
	}
	return v
}

// Request is the form sent by the button by (nil for none): a GET puts the
// values into the address in place of its query; a POST into the body,
// urlencoded or, when the form asks for it, multipart.
func (fm *Form) Request(ctx context.Context, by *Field) (*http.Request, error) {
	values := fm.Values(by)
	if fm.Method != "POST" {
		u := *fm.Action
		u.RawQuery = values.Encode()
		u.Fragment = ""
		return http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	}
	if fm.Enctype == "multipart/form-data" {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		for _, f := range fm.Fields {
			for _, val := range values[f.Name] {
				if err := w.WriteField(f.Name, val); err != nil {
					return nil, err
				}
			}
			delete(values, f.Name)
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", fm.Action.String(), &body)
		if err == nil {
			req.Header.Set("Content-Type", w.FormDataContentType())
		}
		return req, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fm.Action.String(), strings.NewReader(values.Encode()))
	if err == nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req, err
}
