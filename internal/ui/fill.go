package ui

import (
	"regexp"
	"time"
)

// A date, a time, a colour box takes one string in the browser's shape
// — "2026-09-23", "#336699" — and drops one that is not, silently. So
// the popup over it says the shape on its border and refuses a value
// that is not in it (app inputKey), rather than let an edit vanish.

// fillFormat is that shape, in the letters a person reads; empty for a
// box that is typed into instead.
func fillFormat(inputType string) string {
	switch inputType {
	case "date":
		return "YYYY-MM-DD"
	case "datetime-local":
		return "YYYY-MM-DDTHH:MM"
	case "time":
		return "HH:MM"
	case "month":
		return "YYYY-MM"
	case "week":
		return "YYYY-Www"
	case "color":
		return "#rrggbb"
	}
	return ""
}

var (
	weekRe  = regexp.MustCompile(`^\d{4}-W(0[1-9]|[1-4]\d|5[0-3])$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// fillOK reports whether v is in the shape. Seconds are allowed where
// the browser allows them.
func fillOK(shape, v string) bool {
	parses := func(layouts ...string) bool {
		for _, l := range layouts {
			if _, err := time.Parse(l, v); err == nil {
				return true
			}
		}
		return false
	}
	switch shape {
	case "YYYY-MM-DD":
		return parses("2006-01-02")
	case "YYYY-MM-DDTHH:MM":
		return parses("2006-01-02T15:04", "2006-01-02T15:04:05")
	case "HH:MM":
		return parses("15:04", "15:04:05")
	case "YYYY-MM":
		return parses("2006-01")
	case "YYYY-Www":
		return weekRe.MatchString(v)
	case "#rrggbb":
		return colorRe.MatchString(v)
	}
	return true
}
