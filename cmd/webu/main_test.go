package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCommandLine is webu's command line (tdp E1): results on stdout,
// errors on stderr after "webu: ", a wrong use with the usage and exit
// status 2, and anything else left to the TUI.
func TestCommandLine(t *testing.T) {
	run := func(args ...string) (int, bool, string, string) {
		var out, errOut bytes.Buffer
		status, handled := command(args, &out, &errOut)
		return status, handled, out.String(), errOut.String()
	}

	for _, v := range []string{"version", "-v", "--version"} {
		status, handled, out, errOut := run(v)
		if !handled || status != 0 || out != "dev\n" || errOut != "" {
			t.Errorf("%s: status %d handled %v, stdout %q, stderr %q; want dev on stdout, status 0", v, status, handled, out, errOut)
		}
	}
	for _, h := range []string{"help", "-h", "--help"} {
		status, handled, out, errOut := run(h)
		if !handled || status != 0 || out != usage || errOut != "" {
			t.Errorf("%s: status %d, stdout %q, stderr %q; want the usage on stdout", h, status, out, errOut)
		}
	}
	if !strings.Contains(usage, "webu update") {
		t.Errorf("the usage has no webu update:\n%s", usage)
	}

	// A wrong use: where it is wrong, a blank line, the usage, status 2.
	status, handled, out, errOut := run("go.dev", "-x")
	if !handled || status != 2 || out != "" || errOut != "webu: Unknown option \"-x\"\n\n"+usage {
		t.Errorf("-x: status %d, stdout %q, stderr %q", status, out, errOut)
	}

	// update on a build from source says to update it there, status 1,
	// nothing looked up (terminu.SelfUpdate).
	status, handled, out, errOut = run("update")
	if !handled || status != 1 || !strings.HasPrefix(errOut, "webu: ") || !strings.Contains(errOut, "git pull && make install") {
		t.Errorf("update: status %d, stdout %q, stderr %q; want status 1 and how to update a build from source", status, out, errOut)
	}

	// URLs and words are the TUI's.
	for _, args := range [][]string{nil, {"go.dev"}, {"terminal", "browser"}, {"version", "go"}} {
		if _, handled, out, errOut := run(args...); handled || out != "" || errOut != "" {
			t.Errorf("%q: handled %v, stdout %q, stderr %q; want it left to the TUI", args, handled, out, errOut)
		}
	}
}
