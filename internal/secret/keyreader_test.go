package secret

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// mask is the mask character's UTF-8 encoding, so tests can count/compare
// without hardcoding the byte sequence in more than one place.
var mask = []byte(string(maskRune))

func TestFeedPlainASCII(t *testing.T) {
	k := newKeyState()
	out, done, abort := k.feed([]byte("abc"))
	if done || abort {
		t.Fatalf("done=%v abort=%v, want both false", done, abort)
	}
	if k.value() != "abc" {
		t.Fatalf("value = %q, want %q", k.value(), "abc")
	}
	if !bytes.Equal(out, bytes.Repeat(mask, 3)) {
		t.Fatalf("out = %q, want three masks", out)
	}
}

func TestFeedMultiByteUTF8RuneEmitsOneMask(t *testing.T) {
	k := newKeyState()
	// "é" (U+00E9) is 2 bytes in UTF-8.
	out, _, _ := k.feed([]byte("é"))
	if k.value() != "é" {
		t.Fatalf("value = %q, want %q", k.value(), "é")
	}
	if !bytes.Equal(out, mask) {
		t.Fatalf("out = %q, want exactly one mask for one rune", out)
	}
}

func TestFeedEmojiFourByteRuneEmitsOneMask(t *testing.T) {
	k := newKeyState()
	// U+1F600 GRINNING FACE is 4 bytes in UTF-8.
	out, _, _ := k.feed([]byte("😀"))
	if k.value() != "😀" {
		t.Fatalf("value = %q, want %q", k.value(), "😀")
	}
	if !bytes.Equal(out, mask) {
		t.Fatalf("out = %q, want exactly one mask for one rune", out)
	}
}

func TestFeedBackspaceOnNonEmptyErasesLastRuneAndOneMask(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("ab"))
	out, done, abort := k.feed([]byte{0x7F}) // DEL
	if done || abort {
		t.Fatalf("done=%v abort=%v, want both false", done, abort)
	}
	if k.value() != "a" {
		t.Fatalf("value = %q, want %q", k.value(), "a")
	}
	want := []byte("\b \b")
	if !bytes.Equal(out, want) {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestFeedBackspaceViaBSAlsoErases(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("a"))
	out, _, _ := k.feed([]byte{0x08}) // BS
	if k.value() != "" {
		t.Fatalf("value = %q, want empty", k.value())
	}
	want := []byte("\b \b")
	if !bytes.Equal(out, want) {
		t.Fatalf("out = %q, want %q", out, want)
	}
}

func TestFeedBackspaceOnEmptyDoesNotUnderflow(t *testing.T) {
	k := newKeyState()
	out, done, abort := k.feed([]byte{0x7F})
	if done || abort {
		t.Fatalf("done=%v abort=%v, want both false", done, abort)
	}
	if k.value() != "" {
		t.Fatalf("value = %q, want empty", k.value())
	}
	if len(out) != 0 {
		t.Fatalf("out = %q, want no output for backspace on empty input", out)
	}
}

func TestFeedCtrlCAborts(t *testing.T) {
	k := newKeyState()
	// The typing itself must never echo plaintext — checked here rather than
	// against out from the Ctrl-C feed call, which is trivially empty since
	// Ctrl-C returns before any processing (that's finding 4: the old
	// assertion checked a value that could never be non-empty).
	typedOut, _, _ := k.feed([]byte("secret"))
	if bytes.Contains(typedOut, []byte("secret")) {
		t.Fatalf("out while typing = %q, must not contain the plaintext value", typedOut)
	}
	out, done, abort := k.feed([]byte{0x03})
	if !abort {
		t.Fatal("abort = false, want true")
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(out) != 0 {
		t.Fatalf("out on Ctrl-C = %q, want empty", out)
	}
}

func TestFeedCtrlDOnEmptyAborts(t *testing.T) {
	k := newKeyState()
	_, done, abort := k.feed([]byte{0x04})
	if !abort {
		t.Fatal("abort = false, want true")
	}
	if done {
		t.Fatal("done = true, want false")
	}
}

func TestFeedCtrlDOnNonEmptyFinishes(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("abc"))
	_, done, abort := k.feed([]byte{0x04})
	if abort {
		t.Fatal("abort = true, want false")
	}
	if !done {
		t.Fatal("done = false, want true")
	}
	if k.value() != "abc" {
		t.Fatalf("value = %q, want %q", k.value(), "abc")
	}
}

func TestFeedBulkPasteInOneChunk(t *testing.T) {
	k := newKeyState()
	out, done, abort := k.feed([]byte("correct-horse-battery"))
	if done || abort {
		t.Fatalf("done=%v abort=%v, want both false", done, abort)
	}
	if k.value() != "correct-horse-battery" {
		t.Fatalf("value = %q, want %q", k.value(), "correct-horse-battery")
	}
	wantMasks := len([]rune("correct-horse-battery"))
	gotMasks := bytes.Count(out, mask)
	if gotMasks != wantMasks || !bytes.Equal(out, bytes.Repeat(mask, wantMasks)) {
		t.Fatalf("out has %d masks, want %d", gotMasks, wantMasks)
	}
}

func TestFeedRuneSplitAcrossTwoChunks(t *testing.T) {
	k := newKeyState()
	full := []byte("é") // 0xC3 0xA9
	if len(full) != 2 {
		t.Fatalf("test fixture assumption broken: len(%q) = %d, want 2", full, len(full))
	}
	out1, done1, abort1 := k.feed(full[:1])
	if done1 || abort1 {
		t.Fatalf("done=%v abort=%v after partial rune, want both false", done1, abort1)
	}
	if len(out1) != 0 {
		t.Fatalf("out after partial rune = %q, want no output until the rune completes", out1)
	}
	if k.value() != "" {
		t.Fatalf("value after partial rune = %q, want empty", k.value())
	}
	out2, done2, abort2 := k.feed(full[1:])
	if done2 || abort2 {
		t.Fatalf("done=%v abort=%v after completing rune, want both false", done2, abort2)
	}
	if !bytes.Equal(out2, mask) {
		t.Fatalf("out after completing rune = %q, want exactly one mask", out2)
	}
	if k.value() != "é" {
		t.Fatalf("value = %q, want %q", k.value(), "é")
	}
}

func TestFeedCRTerminates(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("abc"))
	_, done, abort := k.feed([]byte{'\r'})
	if abort {
		t.Fatal("abort = true, want false")
	}
	if !done {
		t.Fatal("done = false, want true")
	}
	if k.value() != "abc" {
		t.Fatalf("value = %q, want %q", k.value(), "abc")
	}
}

func TestFeedLFTerminates(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("abc"))
	_, done, abort := k.feed([]byte{'\n'})
	if abort {
		t.Fatal("abort = true, want false")
	}
	if !done {
		t.Fatal("done = false, want true")
	}
	if k.value() != "abc" {
		t.Fatalf("value = %q, want %q", k.value(), "abc")
	}
}

func TestFeedPasteWithEmbeddedNewlineSubmitsAtNewline(t *testing.T) {
	k := newKeyState()
	_, done, abort := k.feed([]byte("tok_abc\ndiscarded-after-newline"))
	if abort {
		t.Fatal("abort = true, want false")
	}
	if !done {
		t.Fatal("done = false, want true")
	}
	if k.value() != "tok_abc" {
		t.Fatalf("value = %q, want %q", k.value(), "tok_abc")
	}
}

func TestFeedNeverAccumulatesControlBytesMaskOrTerminator(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("a"))
	k.feed([]byte{0x7F}) // erase it
	k.feed([]byte("bc"))
	k.feed([]byte{0x08}) // erase 'c'
	k.feed([]byte("d"))
	k.feed([]byte{0x1B, '[', 'D'}) // Left arrow (CSI) — must be fully consumed
	k.feed([]byte{0x09})           // TAB — a bare C0 control byte, ignored
	k.feed([]byte("e"))
	v := k.value()
	for _, b := range []byte{0x7F, 0x08, 0x03, 0x04, '\r', '\n', 0x1B, 0x09} {
		if strings.IndexByte(v, b) >= 0 {
			t.Fatalf("value %q contains control byte 0x%02x", v, b)
		}
	}
	if strings.ContainsRune(v, maskRune) {
		t.Fatalf("value %q contains the mask character", v)
	}
	if v != "bde" {
		t.Fatalf("value = %q, want %q", v, "bde")
	}
}

func TestFeedOutputNeverContainsPlaintext(t *testing.T) {
	k := newKeyState()
	// Multi-byte runes matter here: a buggy implementation that masks
	// per-*byte* rather than per accumulated unit would still pass an
	// ASCII-only version of this test (byte count == rune count for ASCII).
	secretValue := "tok_livé_😀_ABCDEF123456"
	out, _, _ := k.feed([]byte(secretValue))
	if bytes.Contains(out, []byte(secretValue)) {
		t.Fatalf("out = %q, must not contain the plaintext value", out)
	}
	wantMasks := utf8.RuneCountInString(secretValue)
	if !bytes.Equal(out, bytes.Repeat(mask, wantMasks)) {
		t.Fatalf("out = %q, want %d masks (one per rune, not one per byte)", out, wantMasks)
	}
}

func TestFeedBackspaceAfterMultiByteRuneErasesWholeRuneInOneErase(t *testing.T) {
	k := newKeyState()
	k.feed([]byte("é")) // 2-byte rune
	out, done, abort := k.feed([]byte{0x7F})
	if done || abort {
		t.Fatalf("done=%v abort=%v, want both false", done, abort)
	}
	if k.value() != "" {
		t.Fatalf("value = %q, want empty", k.value())
	}
	if !bytes.Equal(out, eraseSeq) {
		t.Fatalf("out = %q, want exactly one erase sequence, not one per byte of the rune", out)
	}
}

func TestFeedRuneSplitAcrossThreeChunks(t *testing.T) {
	k := newKeyState()
	full := []byte("😀") // 0xF0 0x9F 0x98 0x80
	if len(full) != 4 {
		t.Fatalf("test fixture assumption broken: len(%q) = %d, want 4", full, len(full))
	}
	chunks := [][]byte{full[:1], full[1:2], full[2:]}
	var lastOut []byte
	for i, c := range chunks {
		out, done, abort := k.feed(c)
		if done || abort {
			t.Fatalf("done=%v abort=%v on chunk %d, want both false", done, abort, i)
		}
		lastOut = out
	}
	if k.value() != "😀" {
		t.Fatalf("value = %q, want %q", k.value(), "😀")
	}
	if !bytes.Equal(lastOut, mask) {
		t.Fatalf("out after completing rune = %q, want exactly one mask", lastOut)
	}
}

func TestFeedIncompleteUTF8AtEnterSurvivesVerbatim(t *testing.T) {
	k := newKeyState()
	// First two bytes of € (0xE2 0x82 0xAC) — a valid lead byte awaiting
	// continuation bytes that never arrive before Enter.
	k.feed([]byte{0xE2, 0x82})
	_, done, abort := k.feed([]byte{'\r'})
	if abort {
		t.Fatal("abort = true, want false")
	}
	if !done {
		t.Fatal("done = false, want true")
	}
	got := []byte(k.value())
	want := []byte{0xE2, 0x82}
	if !bytes.Equal(got, want) {
		t.Fatalf("value = % x, want % x — an incomplete sequence at Enter must survive, not be dropped", got, want)
	}
}

func TestFeedRoundTripIsByteIdenticalToInput(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"ascii", []byte("correct-horse-battery")},
		{"multibyte-utf8", []byte("café-résumé")},
		{"emoji", []byte("😀🔥✅")},
		{"invalid-byte-sequence", []byte{'a', 0xFF, 'b', 0xC0, 0xC1, 'c'}},
		{"mixed-ascii-utf8-invalid-emoji", append(append([]byte("tok_"), 0xFF), []byte("café😀")...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := newKeyState()
			k.feed(tc.in)
			_, done, abort := k.feed([]byte{'\r'})
			if abort || !done {
				t.Fatalf("done=%v abort=%v, want done=true abort=false", done, abort)
			}
			got := []byte(k.value())
			if !bytes.Equal(got, tc.in) {
				t.Fatalf("value = % x, want % x (byte-identical round trip)", got, tc.in)
			}
		})
	}
}
