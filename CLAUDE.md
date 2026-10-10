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

Each symbol's docblock carries its own rationale; this is the map.

- `main.go` — the CLI: `run`, `runSet`/`runShow`/`runRm`, the shared
  prologues (`parsePathCommand`, `splitPathArgs`, `expandTilde`,
  `refuseBadName`, `statExisting`, `reportMissing`, `refuseNonRegular`) and
  the exit-code/error-message surface.
- `internal/secret` — acquires a value: `FromTTY` (masked prompt on
  `/dev/tty`), `FromCommand` (`sh -c`, stdout). The prompt's
  keystroke accounting is `keyState.feed` in `keyreader.go`, the testable
  core. **Known gap:** an external SIGTERM/SIGINT leaves the terminal raw —
  see `docs/roadmap.md`.
- `internal/store` — destinations: `file.go` (`WriteFile`/`ReadFile`, a
  whole file), `envfile.go` (`SetEnvKey`/`GetEnvKey`/`HasEnvKey`/
  `RemoveEnvKey`, one `.env` key, comment/ordering-preserving). Every write
  goes through `atomic.go`'s `writeFileAtomic`.
- `internal/receipt` — the paste-back block and `Fingerprint`.

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

Which occurrence of a duplicate key a `.env` loader honours is not portable,
so `SetEnvKey` and `GetEnvKey` refuse a duplicate rather than act on a value
that may not be the one in effect. `RemoveEnvKey` deletes every occurrence —
unambiguous, and the escape hatch out of such a file — and `HasEnvKey`
reports presence without erroring, which is all `rm --name` needs.

## Footguns — the rationale lives at the symbol

- Create the temp file at `0600`, then chmod; never the reverse —
  `writeFileAtomic`.
- An existing `.env` keeps its mode; a standalone destination always takes
  `--mode` — `SetEnvKey` and `WriteFile`.
- The `.env` key regex is anchored on the whole key — `keyLine`.

## Release model

`main` only — no release cadence justifies a `dev` trunk for a tool this
small; commit straight to `main`. Semver, `v`-prefixed tags. The tracked
root `VERSION` file is the single source of truth, embedded via
`version.go` (`go:embed`) so `cred version` self-reports without a build
step — never restate the version elsewhere. Cutting a release: bump
`VERSION`, commit, tag `v<VERSION>`, and publish a GitHub release
(`gh release create`) with hand-written notes summarising what shipped.

## Deferred work

`docs/roadmap.md` holds the work that is deliberately not done yet, each
entry with its rationale and unlock condition. Read it before concluding a
gap is an oversight — and add to it, rather than to this file, when a
deferral is the outcome of a change.

## Build & test

```bash
just gate    # gofmt check + go vet + go test — the pre-push gate
```

`.githooks/pre-push` runs `docgraph .` (fail-closed: push is blocked if
`docgraph` isn't installed) plus the diff-scoped `footgun-drift`/
`covers-drift` advisories; `git config core.hooksPath .githooks` wires it
per clone.
