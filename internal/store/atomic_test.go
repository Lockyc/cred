package store

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFileAtomic is exercised directly (same package) because RemoveEnvKey
// always calls it with created=false right after a successful ReadFile of
// the same path, so a Stat failure inside writeFileAtomic can't be provoked
// through RemoveEnvKey's public surface. Calling writeFileAtomic on a path
// that doesn't exist reproduces the same not-created-but-Stat-fails
// situation directly: before the fix, a failed Stat fell through to the
// passed-in mode (0 here), and the renamed file came out 0o000.
func TestWriteFileAtomicOnStatFailureReturnsErrorRatherThanZeroingMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gone.env")
	if err := writeFileAtomic(p, []byte("A=1\n"), 0, false); err == nil {
		t.Fatal("want an error when the not-created path's Stat fails")
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("writeFileAtomic must not leave a file behind when Stat fails on the not-created path")
	}
}
