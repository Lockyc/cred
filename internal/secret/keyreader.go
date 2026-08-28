package secret

import "unicode/utf8"

// maskRune is echoed once per accumulated unit entered at a hidden prompt, in
// place of the unit itself — never the character typed. A silent prompt (the
// previous behaviour, golang.org/x/term's ReadPassword) gives the operator no
// way to tell a registered keystroke from a dropped one; a mask per unit
// does.
const maskRune = '●' // U+25CF BLACK CIRCLE

// eraseSeq is written to erase one mask from the terminal on backspace:
// move back over it, overwrite with a space, move back again.
var eraseSeq = []byte("\b \b")

// escState tracks progress through an ANSI escape sequence (arrow keys,
// function keys) so it can be consumed rather than injecting its printable
// tail into the credential. It persists across feed calls exactly like pend,
// since a sequence can be split across terminal read chunks.
type escState int

const (
	escNone escState = iota
	escSawEsc
	escCSI
	escSS3
)

// keyState is the pure, terminal-free byte-processing core behind FromTTY.
// FromTTY itself needs a real terminal and so cannot be unit-tested; every
// rule about what counts as input, a backspace, an abort, or the end of
// input lives here instead, where it can be.
type keyState struct {
	// units holds the accumulated value, one entry per displayed mask: a
	// complete valid rune's bytes, or a single undecodable byte. Bytes, not
	// runes, so an invalid or binary paste is preserved byte-identical
	// rather than corrupted to U+FFFD — see value().
	units [][]byte
	pend  []byte // bytes of a multi-byte rune split across reads, not yet complete
	esc   escState
}

func newKeyState() *keyState {
	return &keyState{}
}

// value returns the bytes accumulated so far, concatenated in entry order.
// The result is not validated as UTF-8 and may contain whatever invalid or
// binary bytes were actually typed or pasted — the value must be
// byte-identical to the input, never "corrected" toward valid UTF-8. It
// never includes an intercepted control byte (Ctrl-C/D, CR/LF, DEL/BS), a
// consumed ANSI escape sequence (arrow/function keys), a bare C0 control
// byte, the mask character, or a terminator — those never become units.
func (k *keyState) value() string {
	var buf []byte
	for _, u := range k.units {
		buf = append(buf, u...)
	}
	return string(buf)
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

		// Continue an in-progress ANSI escape sequence (CSI: ESC '[' ...
		// final byte in 0x40-0x7E, e.g. arrow keys; SS3: ESC 'O' + one byte,
		// e.g. F1-F4). Neither the ESC nor any byte it swallows is ever
		// masked or accumulated.
		if k.esc != escNone {
			switch k.esc {
			case escSawEsc:
				switch b {
				case '[':
					k.esc = escCSI
					i++
				case 'O':
					k.esc = escSS3
					i++
				default:
					// Not a recognised sequence — the ESC itself is still
					// dropped (it's a C0 control byte), and b is reprocessed
					// normally below rather than swallowed.
					k.esc = escNone
				}
			case escSS3:
				k.esc = escNone
				i++
			case escCSI:
				i++
				if b >= 0x40 && b <= 0x7E {
					k.esc = escNone
				}
			}
			continue
		}

		switch {
		case b == 0x03: // Ctrl-C
			return out, false, true
		case b == 0x04: // Ctrl-D
			if len(k.units) == 0 {
				return out, false, true
			}
			return out, true, false
		case b == '\r' || b == '\n':
			// k.pend is necessarily empty here: it is drained into data at the
			// top of feed, and the only place it is refilled (the FullRune
			// branch below) ends the loop immediately after. A partial
			// multi-byte sequence typed before Enter therefore survives via
			// that drain — the terminator makes it an invalid encoding, so
			// each of its bytes decodes as a one-byte unit and is accumulated
			// verbatim, which is what keeps the value byte-identical to what
			// was entered (see TestFeedIncompleteUTF8AtEnterSurvivesVerbatim).
			return out, true, false
		case b == 0x7F || b == 0x08: // DEL or BS
			if len(k.units) > 0 {
				k.units = k.units[:len(k.units)-1]
				out = append(out, eraseSeq...)
			}
			i++
		case b == 0x1B: // ESC — start of a possible ANSI sequence
			k.esc = escSawEsc
			i++
		case b < 0x20:
			// Any other C0 control byte (e.g. Tab): ignored outright — no
			// mask, no accumulation, and no way for it to reach the file.
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
			if r == utf8.RuneError && size == 1 {
				// An invalid byte, not a genuine encoding of U+FFFD. Keep it
				// verbatim as its own one-byte unit rather than substituting
				// the replacement character — corrupting it here would mean
				// the file ends up holding a different value than was
				// entered, while the mask count told the operator otherwise.
				size = 1
			}
			unit := make([]byte, size)
			copy(unit, data[i:i+size])
			k.units = append(k.units, unit)
			out = append(out, string(maskRune)...)
			i += size
		}
	}
	return out, false, false
}
