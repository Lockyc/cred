---
type: roadmap
links:
  - rel: see-also
    to: CLAUDE.md
    note: the invariants and footguns these deferrals sit against
---

# cred — roadmap

Durable tracking for work that is deliberately *not yet* done. Each entry
names why it was deferred and what would unlock it; nothing here is a wall.
Shipped work leaves this file (git carries the past).

## Restore the terminal on an external signal

`secret.FromTTY` puts the tty in raw mode and restores it via
`defer term.Restore`, which covers every in-process exit path — abort, read
error, success. It does **not** cover a SIGTERM/SIGINT delivered to the
process group from outside: the deferred restore never runs and the terminal
is left raw. (An in-terminal Ctrl-C is *not* this case — `MakeRaw` clears
`ISIG`, so it arrives as byte `0x03` and `keyState.feed` handles it.)

Deferred because the window is a few seconds during an interactive prompt and
a `signal.Notify` handler plus its teardown is real complexity against it.
**Unlock:** the first report of a real occurrence, or any second reason to
own a signal handler.

## Reading a hand-edited `.env` value that carries a trailing comment

`store.GetEnvKey` returns everything after the first `=`, so a hand-written
line such as `API_KEY=sk_abc # prod key` yields the value *plus* the comment.
`cred show --name` then reports a byte count and fingerprint for text no
dotenv loader would hand the application — the one failure mode the receipt
exists to prevent. cred's own writes are unaffected: `quoteEnv` quotes any
value containing `#`.

Not yet resolved because the remedy is a product decision, not a bug fix:
loaders disagree (godotenv and python-dotenv strip an unquoted trailing
comment; a shell `source` does not), which is the same ambiguity the
duplicate-key doctrine answers by refusing rather than guessing.
**Unlock:** a decision on whether a shell-`source`d `.env` is a reader cred
supports — refuse the ambiguous line if it is, strip the comment if it is
not.
