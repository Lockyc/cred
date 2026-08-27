package receipt

import (
	"strings"
	"testing"
)

func TestFingerprintIsStableAnd12Hex(t *testing.T) {
	got := Fingerprint("cal_live_abc123")
	if len(got) != 12 {
		t.Fatalf("len = %d, want 12 (got %q)", len(got), got)
	}
	if got != Fingerprint("cal_live_abc123") {
		t.Fatal("fingerprint is not deterministic")
	}
	if got == Fingerprint("cal_live_abc124") {
		t.Fatal("distinct values produced the same fingerprint")
	}
	for _, c := range got {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("non-hex character %q in %q", c, got)
		}
	}
}

func TestFingerprintNeverContainsTheValue(t *testing.T) {
	secret := "supersecrettoken"
	if strings.Contains(Fingerprint(secret), secret) {
		t.Fatal("fingerprint leaked the value")
	}
}

func TestRenderSetIncludesPasteBackLine(t *testing.T) {
	r := Receipt{Path: "/tmp/k", Mode: "600", Bytes: 41, Fingerprint: "a3f91c04e7b2"}
	out := r.RenderSet()
	for _, want := range []string{"cred: OK", "/tmp/k", "600", "41", "a3f91c04e7b2", "Paste this block back"} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderSet() missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSetOmitsEmptyOptionalFields(t *testing.T) {
	r := Receipt{Path: "/tmp/k", Mode: "600", Bytes: 41, Fingerprint: "a3f91c04e7b2"}
	out := r.RenderSet()
	if strings.Contains(out, "key") || strings.Contains(out, "prefix") {
		t.Fatalf("RenderSet() showed an unset optional field:\n%s", out)
	}
}

func TestRenderSetShowsKeyAndPrefixWhenSet(t *testing.T) {
	r := Receipt{Path: "/tmp/.env", Key: "API_KEY", Mode: "600", Bytes: 8,
		Fingerprint: "a3f91c04e7b2", Prefix: "sk_"}
	out := r.RenderSet()
	if !strings.Contains(out, "API_KEY") {
		t.Fatalf("missing key:\n%s", out)
	}
	if !strings.Contains(out, "sk_ ✓") {
		t.Fatalf("missing verified prefix marker:\n%s", out)
	}
}

func TestRenderShowHasModifiedAndNoPasteLine(t *testing.T) {
	r := Receipt{Path: "/tmp/k", Mode: "600", Bytes: 41,
		Fingerprint: "a3f91c04e7b2", Modified: "2026-08-27 17:04:23"}
	out := r.RenderShow()
	if !strings.Contains(out, "2026-08-27 17:04:23") {
		t.Fatalf("missing modified:\n%s", out)
	}
	if strings.Contains(out, "Paste this block back") {
		t.Fatalf("show must not ask for a paste-back:\n%s", out)
	}
}
