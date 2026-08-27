// Package receipt renders the proof-of-landing block that cred prints after a
// write. The block is the handshake between the user and the agent that asked
// for the credential: the user pastes it back, and the agent learns the path,
// mode, size and fingerprint without either of them seeing the value.
package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Receipt describes what landed. Every field is safe to display.
type Receipt struct {
	Path        string
	Key         string // set only for a .env destination
	Mode        string
	Bytes       int
	Fingerprint string
	Prefix      string // set only when --expect-prefix was given and matched
	Modified    string // set only by show
}

// Fingerprint is a short, unsalted digest prefix of the value. Unsalted so the
// same credential stored in two places is recognisably the same; 12 hex chars
// (48 bits) is enough to detect a changed value and far too short to be a
// useful oracle against a real high-entropy credential.
func Fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func (r Receipt) lines() []string {
	var b []string
	row := func(k, v string) { b = append(b, fmt.Sprintf("  %-12s %s", k, v)) }
	row("path", r.Path)
	if r.Key != "" {
		row("key", r.Key)
	}
	row("mode", r.Mode)
	row("bytes", fmt.Sprintf("%d", r.Bytes))
	row("fingerprint", r.Fingerprint)
	if r.Prefix != "" {
		row("prefix", r.Prefix+" ✓")
	}
	if r.Modified != "" {
		row("modified", r.Modified)
	}
	return b
}

// RenderSet is printed after a successful write.
func (r Receipt) RenderSet() string {
	return "cred: OK\n" + strings.Join(r.lines(), "\n") +
		"\n\nPaste this block back to the agent.\n"
}

// RenderShow is printed by `cred show`. No paste-back prompt: show is a lookup,
// not a handshake.
func (r Receipt) RenderShow() string {
	return "cred: present\n" + strings.Join(r.lines(), "\n") + "\n"
}
