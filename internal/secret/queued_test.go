package secret

import (
	"os"
	"testing"
)

// A read can end exactly at a paste's first line ending, leaving its other
// lines queued on the terminal; FromTTY must see them before flushing them.
func TestQueuedMoreLinesSeesTheRestOfAPasteStillWaiting(t *testing.T) {
	for _, tc := range []struct {
		queued string
		more   bool
	}{
		{"", false},
		{"\n", false},
		{"\r\n", false},
		{"line two\n-----END BLOCK-----\n", true},
	} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.WriteString(tc.queued); err != nil {
			t.Fatal(err)
		}
		if got := queuedMoreLines(r); got != tc.more {
			t.Errorf("%q queued: queuedMoreLines = %v, want %v", tc.queued, got, tc.more)
		}
		r.Close()
		w.Close()
	}
}
