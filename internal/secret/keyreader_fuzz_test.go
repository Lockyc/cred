package secret

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

// feedSplit feeds in to a fresh keyState as two chunks cut at cut, stopping
// at the first chunk that finishes or aborts, the way FromTTY reads.
func feedSplit(in []byte, cut int) (k *keyState, out []byte, done, abort bool) {
	k = newKeyState()
	for _, chunk := range [][]byte{in[:cut], in[cut:]} {
		o, d, a := k.feed(chunk)
		out = append(out, o...)
		if d || a {
			return k, out, d, a
		}
	}
	return k, out, false, false
}

// typedValue is what a prompt fed in, with no Ctrl-C, Ctrl-D, Enter or
// erase byte in it, accumulates: every byte except C0 controls and the
// CSI/SS3 escape sequences arrow and function keys send, and except a
// trailing rune whose remaining bytes have not arrived.
func typedValue(in []byte) []byte {
	var v []byte
	for i := 0; i < len(in); {
		b := in[i]
		switch {
		case b == 0x1B:
			i++
			if i < len(in) && in[i] == '[' {
				for i++; i < len(in) && (in[i] < 0x40 || in[i] > 0x7E); i++ {
				}
				i++
			} else if i < len(in) && in[i] == 'O' {
				i += 2
			}
		case b < 0x20:
			i++
		default:
			if !utf8.FullRune(in[i:]) {
				return v
			}
			v = append(v, b)
			i++
		}
	}
	return v
}

// FuzzFeed: a value split across reads accumulates exactly as it does in one
// read, typed bytes reach the value byte-identical with only controls and
// escape sequences removed, and the terminal is never sent plaintext.
func FuzzFeed(f *testing.F) {
	f.Add([]byte("correct-horse"), uint8(3))
	f.Add([]byte("café😀\r"), uint8(4))
	f.Add([]byte{'a', 0xFF, 'b', 0xC0, 0xE2, 0x82}, uint8(5))
	f.Add([]byte("ab\x1b[1;5Acd\x1bOPef\x1bxg"), uint8(4))
	f.Add([]byte("ab\x1b[1;"), uint8(5))
	f.Add([]byte("x\tz\x7f\x08é\x7fy\x04"), uint8(6))
	f.Add([]byte("\x04"), uint8(0))
	f.Add([]byte("ab\x03cd"), uint8(1))
	f.Fuzz(func(t *testing.T, in []byte, cut uint8) {
		c := int(cut) % (len(in) + 1)
		whole := newKeyState()
		wOut, wDone, wAbort := whole.feed(in)
		split, sOut, sDone, sAbort := feedSplit(in, c)

		if split.value() != whole.value() || sDone != wDone || sAbort != wAbort || !bytes.Equal(sOut, wOut) {
			t.Fatalf("cut at %d: value %q done=%v abort=%v out %q; in one read: value %q done=%v abort=%v out %q",
				c, split.value(), sDone, sAbort, sOut, whole.value(), wDone, wAbort, wOut)
		}

		rest := bytes.ReplaceAll(bytes.ReplaceAll(wOut, eraseSeq, nil), mask, nil)
		if len(rest) != 0 {
			t.Fatalf("terminal output %q holds %q besides masks and erases", wOut, rest)
		}
		if masks, erases := bytes.Count(wOut, mask), bytes.Count(wOut, eraseSeq); masks-erases != len(whole.units) {
			t.Fatalf("%d masks less %d erases shown for %d units", masks, erases, len(whole.units))
		}

		if bytes.ContainsAny(in, "\x03\x04\r\n\x7f\x08") {
			return
		}
		if want := typedValue(in); whole.value() != string(want) {
			t.Fatalf("value % x, want % x", whole.value(), want)
		}
	})
}
