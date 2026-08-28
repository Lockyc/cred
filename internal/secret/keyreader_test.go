package secret

import (
	"bytes"
	"strings"
	"testing"
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
	k.feed([]byte("secret"))
	out, done, abort := k.feed([]byte{0x03})
	if !abort {
		t.Fatal("abort = false, want true")
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if bytes.Contains(out, []byte("secret")) {
		t.Fatalf("out = %q, must not contain the plaintext value", out)
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
	if !bytes.Equal(out, bytes.Repeat(mask, wantMasks)) {
		t.Fatalf("out has %d masks, want %d", bytes.Count(out, mask), wantMasks)
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
	v := k.value()
	for _, b := range []byte{0x7F, 0x08, 0x03, 0x04, '\r', '\n'} {
		if strings.IndexByte(v, b) >= 0 {
			t.Fatalf("value %q contains control byte 0x%02x", v, b)
		}
	}
	if strings.ContainsRune(v, maskRune) {
		t.Fatalf("value %q contains the mask character", v)
	}
	if v != "bd" {
		t.Fatalf("value = %q, want %q", v, "bd")
	}
}

func TestFeedOutputNeverContainsPlaintext(t *testing.T) {
	k := newKeyState()
	secretValue := "tok_live_ABCDEF123456"
	out, _, _ := k.feed([]byte(secretValue))
	if bytes.Contains(out, []byte(secretValue)) {
		t.Fatalf("out = %q, must not contain the plaintext value", out)
	}
	// Every byte written to the terminal must be part of a mask rune.
	for i := 0; i < len(out); i += len(mask) {
		if !bytes.Equal(out[i:i+len(mask)], mask) {
			t.Fatalf("out contains a non-mask byte sequence at offset %d: %q", i, out)
		}
	}
}
