---
type: architecture
links:
  - rel: see-also
    to: README.md
    note: human-facing counterpart — install, usage, and the receipt, explained for the person typing the value
---

# cred — notes for the next agent

A small Go CLI: intake a credential (typed hidden, or read from a command's
stdout) and land it at a destination — a standalone file, or one key inside a
`.env`. Scope boundary: **intake and destination only.** Not encryption at
rest, not a password manager, not "run this command with the secret in its
environment." Those are other tools' jobs — see the README's "what cred
deliberately does not do."

## Package layout

- `main.go` — the CLI: flag parsing (`splitPathArgs` lets a flag appear
  before, after, or between the path — `flag.FlagSet.Parse` alone stops at
  the first non-flag token, which would otherwise require every flag to
  precede the path), `runSet`/`runShow`/`runRm`, `expandTilde` (handles a
  quoted leading `~/`, since an unquoted `~` is already expanded by the
  shell before cred ever sees it), and the exit-code/error-message surface.
  `reportMissing` and `refuseNonRegular` are shared by all three commands so
  they stay in agreement: an absent path is `Lstat`'s `os.ErrNotExist`,
  reported as `MISSING` on stdout (any other `Lstat` error — EACCES on a
  parent dir, ELOOP, … — is a runtime error on stderr instead, since
  reporting it as `MISSING` would point someone at `cred set` to "fix" a
  credential that is actually present but merely unreadable, overwriting
  it); a path that exists but isn't a regular file — a directory, or a
  symlink judged by `Lstat` rather than resolved to its target — is refused
  before `set` would `Rename` onto it, `show` would `read` it, or `rm` would
  unlink it. `runRm --name` checks `store.HasEnvKey` before calling
  `store.RemoveEnvKey`, because `RemoveEnvKey` rewrites the file
  unconditionally (new inode, bumped mtime) even when the key was already
  absent; gating on presence first is what lets "nothing removed" (exit 1)
  also mean "nothing was touched." `expandTilde`'s `~user` rejection carries
  a sentinel error (`errUnsupportedTildeUser`) distinct from a genuine
  `os.UserHomeDir` failure, so `parsePathCommand` can return the former as a
  usage error (2) and the latter as a runtime error (1) despite sharing one
  return path.
- `internal/secret` — acquires a value. `FromTTY` reads `/dev/tty` directly
  (not stdin), so prompting still works when stdin is a pipe; `FromCommand`
  runs `sh -c <cmd>` and takes its stdout. Both trim a trailing `\r\n` and
  reject an empty result (`ErrEmpty`) — an empty write is always a mis-paste
  or a failed command, never intentional. `FromTTY` puts the terminal in raw
  mode and echoes one `●` per rune entered rather than staying silent — a
  fully silent prompt (the prior behaviour, `golang.org/x/term`'s
  `ReadPassword`) gives the operator no way to tell a registered keystroke
  from a dropped one, which defeats a tool whose whole point is confirming a
  value landed. Ctrl-C, and Ctrl-D on empty input, abort (`ErrAborted`) and
  write nothing; the terminal is restored via `defer term.Restore`
  immediately after `term.MakeRaw`, so every exit path — abort, a mid-read
  error, success — leaves the terminal usable afterwards. All the actual
  rune-counting/backspace/abort/terminator logic lives in
  `keyreader.go`'s `keyState.feed`, a pure function with no terminal
  dependency, precisely because `FromTTY` itself needs a real terminal and
  so cannot be unit-tested — `keyState` is what gives that logic coverage at
  all.
- `internal/store` — `file.go` (`WriteFile`/`ReadFile`, a whole-file
  destination) and `envfile.go` (`SetEnvKey`/`GetEnvKey`/`HasEnvKey`/
  `RemoveEnvKey`, a single `.env` key, comment/ordering-preserving). Both
  route writes through `atomic.go`'s `writeFileAtomic` (temp file in the
  same directory, `Sync`, then `Rename`), so a crash mid-write never
  truncates the original.
- `internal/receipt` — renders the paste-back block. `Fingerprint` is an
  unsalted 12-hex-char SHA-256 prefix: unsalted so the same credential in two
  places is recognisably the same value, short enough that it is not a
  useful guessing oracle against a real high-entropy secret.

## The invariant

A credential value may reach exactly one place cred itself writes it to: a
destination file. It arrives in cred's own memory one of two ways — typed at
a hidden prompt read directly from `/dev/tty` (`FromTTY`), or captured from
a child command's **stdout** through a pipe (`FromCommand`, via `sh -c
<cmd>`). cred never writes a value *into* a child process — `FromCommand`
only ever reads one back out. It must never reach cred's own argv, a log, or
stdout. **`cred get` was considered and deliberately rejected** — printing a
value to stdout is exactly how it ends up in a shell history, a log, or an
agent's transcript. Do not add it. `cred show`/`--name` report path, mode,
size, and fingerprint — never the value.

The one place this invariant is at the user's mercy rather than cred's: the
*command string* passed to `--value-from` must itself fetch the value (`op
read op://vault/item/field`), never contain it (`echo sk-live-...`) — a
command that contains the credential puts it in cred's own `os.Args` before
`FromCommand` ever runs, which no amount of care inside `FromCommand` can
undo.

## Duplicate `.env` keys are refused, not guessed (design decision)

`SetEnvKey` and `GetEnvKey` (`internal/store/envfile.go`) both refuse with an
error, writing/reading nothing, when a key appears more than once in the
file. Which occurrence a `.env` loader honours on a duplicate key is not
portable across loaders — writing into (or reading from) one occurrence
risks silently acting on a value that isn't the one actually in effect, and
that's the worst failure mode for a credential tool. `RemoveEnvKey` is the
one exception: it deletes every occurrence, because that outcome is
unambiguous, and it's the documented escape hatch out of a duplicate-key
file the other two functions won't touch. `HasEnvKey` (used by `rm --name`
to decide whether there's anything to remove) also doesn't error on a
duplicate, for the same reason: presence, not a single authoritative value,
is all it needs to answer.

## Footgun — create at 0600, then chmod to the target mode; never the reverse

`writeFileAtomic` (`internal/store/atomic.go`) creates the temp file via
`os.CreateTemp` (which opens at `0600`), writes and syncs it, *then*
`os.Chmod`s it to the target mode before the rename. The umask can only
**remove** bits from what a process requests, never add them — so creating
at `0600` first guarantees there is never a window where the file is wider
than intended, regardless of the caller's umask. Reversing the order
(`Chmod` to the wide target mode first, then write) reintroduces exactly
that window: a file briefly readable by the group or world before it's
tightened, and the crash/interrupt case fails open instead of closed.

## Footgun — the `.env` mode rule: an existing file keeps its mode

`--mode` applies only when the file is being **created**
(`writeFileAtomic`'s `created` flag). An existing `.env` keeps the mode it
already has — it may legitimately be `644` (a `.env.example`-shaped file
under version control that other tooling reads), and cred silently
tightening it on a `set --name` would be a silent breakage of whatever else
reads that file. A standalone destination (`store.WriteFile`) is the
opposite on purpose: it always enforces the requested mode, even on an
existing path, because the whole point of a standalone destination is that
the caller picked its mode deliberately.

## Footgun — the `.env` key regex is anchored on the whole key

`keyLine` (`internal/store/envfile.go`) matches
`^\s*(export\s+)?<key>=` with the key fully escaped and anchored — so a
lookup for `API_KEY` never matches a line beginning `MY_API_KEY=`. An
unanchored or prefix-only match would silently operate on the wrong
variable in any `.env` where one key is a suffix of another.

## Release model

`main` only — no release cadence justifies a `dev` trunk for a tool this
small; commit straight to `main`. Semver, `v`-prefixed tags. The tracked
root `VERSION` file is the single source of truth, embedded via
`version.go` (`go:embed`) so `cred version` self-reports without a build
step — never restate the version elsewhere. Cutting a release: bump
`VERSION`, commit, tag `v<VERSION>`, and publish a GitHub release
(`gh release create`) with hand-written notes summarising what shipped.

## Build & test

```bash
just gate    # gofmt check + go vet + go test — the pre-push gate
```

`.githooks/pre-push` runs `docgraph .` (fail-closed: push is blocked if
`docgraph` isn't installed) plus the diff-scoped `footgun-drift`/
`covers-drift` advisories; `git config core.hooksPath .githooks` wires it
per clone.
