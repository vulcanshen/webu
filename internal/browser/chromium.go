package browser

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Revision is the one Chromium webu runs. Pinned, not overridable: the
// translation layer's heuristics and every role fixture are answered for
// against this build and no other (function.md §9). Bumping it is a decision
// — `webu browser update` — not a side effect of somebody's PATH.
//
// 1701465 is a main-branch snapshot (V8 15.6, Chromium 156 line) present for
// all three platforms below; a revision missing on one of them is not a
// candidate, because the fixture promise is the same on every machine.
const Revision = 1701465

// snapshotHost is where the Chromium project publishes per-commit builds.
// Overridable for tests only.
var snapshotHost = "https://storage.googleapis.com/chromium-browser-snapshots"

// platform is one row of the snapshot layout: the bucket directory, the zip
// name, and where the executable sits inside the zip.
type platform struct {
	dir string // bucket directory, e.g. Mac_Arm
	zip string // archive name, e.g. chrome-mac.zip
	exe string // executable path inside the archive
}

// macExe is the same for both Mac buckets: an app bundle, and the binary is
// inside it. It is run directly — no `open`, no Launch Services — which is
// what keeps it windowless and out of the Dock.
const macExe = "chrome-mac/Chromium.app/Contents/MacOS/Chromium"

// platforms is every GOOS/GOARCH webu can fetch a Chromium for. Linux arm64 is
// absent because the snapshot bucket has no such build; that machine gets an
// error naming the gap rather than a download that cannot run.
var platforms = map[string]platform{
	"darwin/arm64": {"Mac_Arm", "chrome-mac.zip", macExe},
	"darwin/amd64": {"Mac", "chrome-mac.zip", macExe},
	"linux/amd64":  {"Linux_x64", "chrome-linux.zip", "chrome-linux/chrome"},
}

// ErrUnsupportedPlatform says there is no snapshot for this machine.
var ErrUnsupportedPlatform = errors.New("no Chromium snapshot for this platform")

func platformFor(goos, goarch string) (platform, error) {
	p, ok := platforms[goos+"/"+goarch]
	if !ok {
		return platform{}, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}
	return p, nil
}

func hostPlatform() (platform, error) { return platformFor(runtime.GOOS, runtime.GOARCH) }

// snapshotURL is the archive for one platform at the pinned revision.
func snapshotURL(p platform) string {
	return snapshotHost + "/" + p.dir + "/" + strconv.Itoa(Revision) + "/" + p.zip
}

// InstallDir is the directory the pinned revision unpacks into. The revision
// is in the name so an update never overwrites a build that is still running.
func InstallDir() (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "chromium-"+strconv.Itoa(Revision)), nil
}

// Executable is where the pinned Chromium's binary is, or would be. It does
// not check that it exists — Installed does.
func Executable() (string, error) {
	p, err := hostPlatform()
	if err != nil {
		return "", err
	}
	dir, err := InstallDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(p.exe)), nil
}

// Installed reports the binary's path and whether it is there to run.
func Installed() (string, bool) {
	exe, err := Executable()
	if err != nil {
		return "", false
	}
	st, err := os.Stat(exe)
	return exe, err == nil && !st.IsDir()
}

// Progress is one download report: bytes so far, and the total if the server
// said (or -1 if it did not).
type Progress struct{ Done, Total int64 }

// Download fetches and unpacks the pinned revision for this machine and
// returns the executable's path. progress may be nil.
//
// The archive streams to a .part file and unpacks into a .tmp directory; the
// final directory only appears once everything inside it is in place, so a
// download killed halfway leaves nothing that Installed would mistake for a
// browser. Re-running picks up from nothing — the archive is not resumed, it
// is fetched again, which is simpler than being right about ranges.
func Download(ctx context.Context, progress func(Progress)) (string, error) {
	p, err := hostPlatform()
	if err != nil {
		return "", err
	}
	dir, err := InstallDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	archive := dir + ".zip.part"
	if err := fetch(ctx, snapshotURL(p), archive, progress); err != nil {
		os.Remove(archive)
		return "", err
	}
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := unzip(archive, tmp); err != nil {
		os.RemoveAll(tmp)
		os.Remove(archive)
		return "", err
	}
	os.Remove(archive)
	os.RemoveAll(dir)
	if err := os.Rename(tmp, dir); err != nil {
		return "", err
	}
	exe := filepath.Join(dir, filepath.FromSlash(p.exe))
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("archive unpacked but %s is not in it", p.exe)
	}
	return exe, nil
}

// ArchiveSize asks the server how big the download is, so the first run can
// say the number before committing to it (function.md §12). -1 when unknown.
func ArchiveSize(ctx context.Context) (int64, error) {
	p, err := hostPlatform()
	if err != nil {
		return -1, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, snapshotURL(p), nil)
	if err != nil {
		return -1, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("%s: %s", snapshotURL(p), resp.Status)
	}
	return resp.ContentLength, nil
}

// fetch streams url to path, reporting as it goes.
func fetch(ctx context.Context, url, path string, progress func(Progress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(Progress{Done: done, Total: total})
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// unzip unpacks archive into dir.
//
// Symlinks are recreated as symlinks. The Mac bundle depends on it: a
// framework's Versions/Current is a link, and a copy of the target in its
// place is a bundle that does not launch. archive/zip reports a link as a file
// whose mode has ModeSymlink and whose content is the target path.
func unzip(archive, dir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		dst, err := safeJoin(dir, f.Name)
		if err != nil {
			return err
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			target, err := readAll(f)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(string(target), dst); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := writeFile(f, dst, mode.Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func writeFile(f *zip.File, dst string, perm os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	// A file with no permission bits at all (some zip writers) still has to be
	// readable, and an executable has to stay one.
	if perm&0o400 == 0 {
		perm |= 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// safeJoin refuses an archive entry that would land outside dir.
func safeJoin(dir, name string) (string, error) {
	dst := filepath.Join(dir, filepath.FromSlash(name))
	if dst != dir && !strings.HasPrefix(dst, dir+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes the target directory: %q", name)
	}
	return dst, nil
}
