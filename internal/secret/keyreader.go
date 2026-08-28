package secret

import "unicode/utf8"

// maskRune is echoed once per rune entered at a hidden prompt, in place of
// the rune itself — never the character typed. A silent prompt (the previous
// behaviour, golang.org/x/term's ReadPassword) gives the operator no way to
// tell a registered keystroke from a dropped one; a mask per rune does.
const maskRune = '●' // U+25CF BLACK CIRCLE

// eraseSeq is written to erase one mask from the terminal on backspace:
// move back over it, overwrite with a space, move back again.
var eraseSeq = []byte("\b \b")

// keyState is the pure, terminal-free byte-processing core behind FromTTY.
// FromTTY itself needs a real terminal and so cannot be unit-tested; every
// rule about what counts as input, a backspace, an abort, or the end of
// input lives here instead, where it can be.
type keyState struct {
	runes []rune
	pend  []byte // bytes of a multi-byte rune split across reads, not yet complete
}

func newKeyState() *keyState {
	return &keyState{}
}

// value returns the runes accumulated so far. It never includes a control
// byte, the mask character, or a terminator — those are all consumed by feed
// without being appended to runes.
func (k *keyState) value() string {
	return string(k.runes)
}

// feed processes one chunk of raw terminal bytes (raw mode: no line
// buffering, no OS-level echo, ISIG disabled so Ctrl-C arrives as 0x03
// rather than a signal). It returns the mask/erase bytes to write back to
// the terminal, whether input is complete (Enter, or Ctrl-D on non-empty
// input), and whether it was aborted (Ctrl-C, or Ctrl-D on empty input).
//
// On done or abort, feed returns immediately — any bytes in chunk after the
// triggering byte are discarded, matching the existing behaviour of a
// pasted value containing a newline: it submits at the newline.
func (k *keyState) feed(chunk []byte) (out []byte, done bool, abort bool) {
	data := chunk
	if len(k.pend) > 0 {
		data = append(k.pend, chunk...)
		k.pend = nil
	}

	i := 0
	for i < len(data) {
		b := data[i]
		switch {
		case b == 0x03: // Ctrl-C
			return out, false, true
		case b == 0x04: // Ctrl-D
			if len(k.runes) == 0 {
				return out, false, true
			}
			return out, true, false
		case b == '\r' || b == '\n':
			return out, true, false
		case b == 0x7F || b == 0x08: // DEL or BS
			if len(k.runes) > 0 {
				k.runes = k.runes[:len(k.runes)-1]
				out = append(out, eraseSeq...)
			}
			i++
		default:
			if !utf8.FullRune(data[i:]) {
				// A multi-byte rune's lead byte(s) landed at the end of this
				// chunk with its continuation byte(s) not read yet — a real
				// read-boundary case, not just theoretical. Hold what we have
				// and wait for the rest on the next feed.
				k.pend = append(k.pend, data[i:]...)
				i = len(data)
				break
			}
			r, size := utf8.DecodeRune(data[i:])
			k.runes = append(k.runes, r)
			out = append(out, string(maskRune)...)
			i += size
		}
	}
	return out, false, false
}
