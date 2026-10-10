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
