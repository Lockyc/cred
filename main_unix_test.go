//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// pathState is what a destination check must leave untouched at one path.
type pathState struct {
	mode    os.FileMode
	modTime time.Time
	link    string
	body    string
}

func snapshot(t *testing.T, paths []string) map[string]pathState {
	t.Helper()
	states := map[string]pathState{}
	for _, p := range paths {
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("snapshot %s: %v", p, err)
		}
		s := pathState{mode: fi.Mode(), modTime: fi.ModTime()}
		if fi.Mode()&os.ModeSymlink != 0 {
			s.link, _ = os.Readlink(p)
		}
		if fi.Mode().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			s.body = string(b)
		}
		states[p] = s
	}
	return states
}

// destKind builds one kind of destination in dir and returns the path cred
// is given, the stderr text each command must refuse it with, and the paths
// whose state must survive the refusal.
type destKind struct {
	name  string
	build func(t *testing.T, dir string) (dest string, keep []string)
	want  string
}

func destKinds() []destKind {
	const notRegular = "not a regular file"
	file := func(t *testing.T, p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("K=cal_live_secret\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	symlink := func(t *testing.T, target, link string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	return []destKind{
		{"symlink to a file", func(t *testing.T, dir string) (string, []string) {
			target, link := filepath.Join(dir, "target"), filepath.Join(dir, "link")
			file(t, target)
			symlink(t, target, link)
			return link, []string{target, link}
		}, notRegular},
		{"dangling symlink", func(t *testing.T, dir string) (string, []string) {
			link := filepath.Join(dir, "link")
			symlink(t, filepath.Join(dir, "absent"), link)
			return link, []string{link}
		}, notRegular},
		{"directory", func(t *testing.T, dir string) (string, []string) {
			d := filepath.Join(dir, "d")
			if err := os.Mkdir(d, 0o700); err != nil {
				t.Fatal(err)
			}
			return d, []string{d}
		}, notRegular},
		{"fifo", func(t *testing.T, dir string) (string, []string) {
			p := filepath.Join(dir, "fifo")
			if err := syscall.Mkfifo(p, 0o600); err != nil {
				t.Fatal(err)
			}
			return p, []string{p}
		}, notRegular},
		{"symlink loop", func(t *testing.T, dir string) (string, []string) {
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			symlink(t, b, a)
			symlink(t, a, b)
			return a, []string{a, b}
		}, notRegular},
		{"path through a symlink loop (ELOOP)", func(t *testing.T, dir string) (string, []string) {
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			symlink(t, b, a)
			symlink(t, a, b)
			return filepath.Join(a, ".env"), []string{a, b}
		}, "too many levels of symbolic links"},
		{"path under a regular file (ENOTDIR)", func(t *testing.T, dir string) (string, []string) {
			f := filepath.Join(dir, "file")
			file(t, f)
			return filepath.Join(f, ".env"), []string{f}
		}, "not a directory"},
		{"path under a 000 directory (EACCES)", func(t *testing.T, dir string) (string, []string) {
			if os.Geteuid() == 0 {
				t.Skip("running as root: directory permission bits don't block stat")
			}
			d := filepath.Join(dir, "locked")
			if err := os.Mkdir(d, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(d, 0o755) })
			return filepath.Join(d, ".env"), []string{d}
		}, "permission denied"},
	}
}

// set, show and rm, with and without --name, refuse every destination that
// is not a regular file, or that cannot be stat'ed, in cred's own words, and
// leave it as it was. set refuses before it acquires the value.
func TestEveryCommandRefusesEveryNonRegularDestination(t *testing.T) {
	commands := [][]string{
		{"set"}, {"set", "--name", "K"},
		{"show"}, {"show", "--name", "K"},
		{"rm"}, {"rm", "--name", "K"},
	}
	for _, kind := range destKinds() {
		for _, cmd := range commands {
			t.Run(kind.name+"/"+strings.Join(cmd, " "), func(t *testing.T) {
				dir := t.TempDir()
				dest, keep := kind.build(t, dir)
				before := snapshot(t, keep)
				acquired := filepath.Join(dir, "acquired")
				args := append(append([]string{}, cmd...), dest)
				if cmd[0] == "set" {
					args = append(args, "--value-from", "touch '"+acquired+"'; printf cal_live_x")
				}

				var out, errOut bytes.Buffer
				if code := run(args, &out, &errOut); code != 1 {
					t.Fatalf("exit = %d, want 1 (stdout %q, stderr %q)", code, out.String(), errOut.String())
				}
				if out.Len() != 0 {
					t.Errorf("stdout = %q, want nothing: the destination is not missing and nothing happened", out.String())
				}
				if !strings.HasPrefix(errOut.String(), "cred: ") || !strings.Contains(errOut.String(), kind.want) {
					t.Errorf("stderr = %q, want cred's refusal naming %q", errOut.String(), kind.want)
				}
				if _, err := os.Lstat(acquired); err == nil {
					t.Error("the value was acquired before the destination was refused")
				}
				for p, was := range before {
					if now := snapshot(t, []string{p})[p]; now != was {
						t.Errorf("%s changed: %+v -> %+v", p, was, now)
					}
				}
			})
		}
	}
}
