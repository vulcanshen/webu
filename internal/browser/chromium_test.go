package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		dir, exe     string
		ok           bool
	}{
		{"darwin", "arm64", "Mac_Arm", macExe, true},
		{"darwin", "amd64", "Mac", macExe, true},
		{"linux", "amd64", "Linux_x64", "chrome-linux/chrome", true},
		{"linux", "arm64", "", "", false},
		{"windows", "amd64", "", "", false},
	}
	for _, c := range cases {
		p, err := platformFor(c.goos, c.goarch)
		if c.ok != (err == nil) {
			t.Errorf("%s/%s: ok=%v err=%v", c.goos, c.goarch, c.ok, err)
			continue
		}
		if !c.ok {
			continue
		}
		if p.dir != c.dir || p.exe != c.exe {
			t.Errorf("%s/%s: got %+v", c.goos, c.goarch, p)
		}
	}
}

func TestSnapshotURL(t *testing.T) {
	p, _ := platformFor("darwin", "arm64")
	want := "https://storage.googleapis.com/chromium-browser-snapshots/Mac_Arm/1701465/chrome-mac.zip"
	if got := snapshotURL(p); got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

// buildArchive is a tiny stand-in for chrome-mac.zip: a directory, an
// executable, and a symlink — the three kinds of entry the real one has.
func buildArchive(t *testing.T, exe string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add := func(name string, mode os.FileMode, body string) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add(filepath.Dir(exe)+"/", os.ModeDir|0o755, "")
	add(exe, 0o755, "#!/bin/sh\necho chromium\n")
	add("chrome-mac/Versions/1.0/lib", 0o644, "lib")
	add("chrome-mac/Versions/Current", os.ModeSymlink|0o777, "1.0")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnzipKeepsSymlinksAndModes(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(archive, buildArchive(t, "chrome-mac/bin/chromium"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "out")
	if err := unzip(archive, dir); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "chrome-mac/bin/chromium"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Errorf("executable lost its x bit: %v", st.Mode())
	}
	link, err := os.Readlink(filepath.Join(dir, "chrome-mac/Versions/Current"))
	if err != nil {
		t.Fatalf("symlink not recreated: %v", err)
	}
	if link != "1.0" {
		t.Errorf("symlink target %q", link)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "chrome-mac/Versions/Current/lib")); err != nil || string(b) != "lib" {
		t.Errorf("through the link: %q %v", b, err)
	}
}

func TestUnzipRefusesEscape(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("../evil")
	f.Write([]byte("x"))
	w.Close()
	archive := filepath.Join(t.TempDir(), "a.zip")
	os.WriteFile(archive, buf.Bytes(), 0o644)
	if err := unzip(archive, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("an entry escaping the directory was unpacked")
	}
}

func TestDownloadInstallsAndReports(t *testing.T) {
	p, err := hostPlatform()
	if err != nil {
		t.Skip(err)
	}
	body := buildArchive(t, p.exe)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+p.dir+"/1701465/"+p.zip {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()
	old := snapshotHost
	snapshotHost = srv.URL
	defer func() { snapshotHost = old }()
	t.Setenv("WEBU_CACHE", t.TempDir())

	if _, ok := Installed(); ok {
		t.Fatal("installed before download")
	}
	var last Progress
	exe, err := Download(context.Background(), func(pr Progress) { last = pr })
	if err != nil {
		t.Fatal(err)
	}
	if last.Done != int64(len(body)) || last.Total != int64(len(body)) {
		t.Errorf("progress ended at %+v, archive is %d bytes", last, len(body))
	}
	got, ok := Installed()
	if !ok || got != exe {
		t.Errorf("Installed() = %q,%v; Download returned %q", got, ok, exe)
	}
	dir, _ := InstallDir()
	if _, err := os.Stat(dir + ".zip.part"); err == nil {
		t.Error("the archive was left behind")
	}
	if _, err := os.Stat(dir + ".tmp"); err == nil {
		t.Error("the staging directory was left behind")
	}
}

func TestDownloadFailsCleanly(t *testing.T) {
	if _, err := hostPlatform(); err != nil {
		t.Skip(err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	old := snapshotHost
	snapshotHost = srv.URL
	defer func() { snapshotHost = old }()
	t.Setenv("WEBU_CACHE", t.TempDir())
	if _, err := Download(context.Background(), nil); err == nil {
		t.Fatal("a 404 was reported as success")
	}
	if _, ok := Installed(); ok {
		t.Error("a failed download left something Installed() believes in")
	}
}

func TestProfileDirUnderConfig(t *testing.T) {
	t.Setenv("WEBU_CONFIG", "/tmp/wc")
	if d, _ := ProfileDir(); d != filepath.Join("/tmp/wc", "profile") {
		t.Errorf("ProfileDir %q", d)
	}
}
