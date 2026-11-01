//go:build unix

package store

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func fileGid(fi os.FileInfo) int { return int(fi.Sys().(*syscall.Stat_t).Gid) }

// A rewrite keeps the file's group, not the writer's primary group. Needs a
// second group the user belongs to; skipped where there is none.
func TestWriteFileAtomicKeepsTheExistingGroup(t *testing.T) {
	groups, err := os.Getgroups()
	if err != nil {
		t.Skip("no group list")
	}
	other := -1
	for _, g := range groups {
		if g != os.Getgid() {
			other = g
			break
		}
	}
	if other < 0 {
		t.Skip("the user belongs to only one group")
	}
	p := filepath.Join(t.TempDir(), "shared.env")
	if err := os.WriteFile(p, []byte("A=1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(p, -1, other); err != nil {
		t.Skipf("cannot set the test file's group: %v", err)
	}
	if err := writeFileAtomic(p, []byte("A=2\n"), 0o600, false); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileGid(fi); got != other {
		t.Fatalf("group after rewrite = %d, want %d", got, other)
	}
}
