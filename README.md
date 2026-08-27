# cred

![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)
[![CI](https://github.com/lockyc/cred/actions/workflows/ci.yml/badge.svg)](https://github.com/lockyc/cred/actions/workflows/ci.yml)

A command that puts a credential into a file, without ever showing it to
anyone who didn't type it.

## Why a command, not a shell snippet

An agent that needs you to place a credential — an API key, a token — has two
ways to ask: hand you shell text to paste, or run a command. Shell text is
dialect-specific. A pasteable built from `read -sp`, `umask`, and `printf`
has to be written in *your* interactive shell's dialect, and a snippet
written for bash silently breaks in fish — different builtins, different
quoting, sometimes no error at all, just a wrong file. A bare command has no
dialect to get wrong: `cred set <path>` means the same thing in bash, fish,
zsh, or typed by hand. That's the whole reason this tool exists.

## Install

```bash
go install github.com/lockyc/cred@latest
```

Or, without a local Go toolchain setup step of your own:

```bash
curl -fsSL https://raw.githubusercontent.com/lockyc/cred/main/install.sh | bash
```

Both need Go on `PATH`; the curl form just wraps the same `go install` and
prints where the binary landed.

## Usage

```bash
cred set  <path> [--name KEY] [--expect-prefix P] [--mode M] [--value-from CMD]
cred show <path> [--name KEY]
cred rm   <path> [--name KEY]
cred version
```

Flags may appear before or after the path (`cred set --name KEY <path>` and
`cred set <path> --name KEY` both work). Run `cred <command> -h` for a
command's own option list.

**`set`** prompts on the controlling terminal with echo disabled (reads
`/dev/tty`, so it works even when stdin is piped) and writes the value to
`<path>`. `--value-from '<command>'` reads the value from another command's
stdout instead of prompting — useful when scripting. `--expect-prefix`
refuses to write if the value doesn't start with the given string, catching
a wrong paste before it lands. `--mode` sets the octal file mode of a
newly-created file (default `600`).

Worked example — a standalone credential file:

```console
$ cred set ~/.config/example/token --expect-prefix tok_
Value for /home/you/.config/example/token: [hidden]
cred: OK
  path         /home/you/.config/example/token
  mode         600
  bytes        26
  fingerprint  97d90e6a6af2
  prefix       tok_ ✓

Paste this block back to the agent.
```

```console
$ cred show ~/.config/example/token
cred: present
  path         /home/you/.config/example/token
  mode         600
  bytes        26
  fingerprint  97d90e6a6af2
  modified     2026-08-28 08:35:11

$ cred rm ~/.config/example/token
cred: removed /home/you/.config/example/token
```

**`--name KEY`** redirects `set`/`show`/`rm` at one key inside a `.env`-style
file instead of the whole file — the rest of the file (comments, ordering,
every other key) is left untouched:

```console
$ cred set ~/project/.env --name API_KEY --expect-prefix sk_
Value for API_KEY: [hidden]
cred: OK
  path         /home/you/project/.env
  key          API_KEY
  mode         600
  bytes        28
  fingerprint  46c0a14c6781
  prefix       sk_ ✓

Paste this block back to the agent.
```

`cred rm ~/project/.env --name API_KEY` removes just that line; on a key
that isn't set, it reports nothing was removed and exits 1 without touching
the file.

### The receipt

Every successful `set`/`show` prints a receipt, and `set` asks you to paste
it back to the agent that requested the credential:

```
cred: OK
  path         /home/you/.config/example/token
  mode         600
  bytes        26
  fingerprint  97d90e6a6af2
  prefix       tok_ ✓

Paste this block back to the agent.
```

The agent learns the path, file mode, byte count, and a 12-hex-character
fingerprint (an unsalted SHA-256 prefix — long enough to confirm two
receipts refer to the same value, far too short to be a useful guess against
a real credential) — everything it needs to confirm the write landed, and
nothing it needs to see the value itself. Neither party ever has the
credential and the receipt in the same place.

### Duplicate keys in a `.env` file are refused, not guessed

If a key appears more than once in a `.env` file, `set --name` and
`show --name` both refuse with an error and change nothing. Which
occurrence a `.env` loader honours on a duplicate key is not portable across
loaders, so writing into (or reading from) one occurrence risks silently
acting on a value that isn't the one actually in effect. `rm --name` is the
escape hatch: it deletes every occurrence of the key, and you re-add it
clean with `set`.

### Conventions, not enforcement

A standalone credential conventionally lives at `~/.config/<service>/<name>`,
but `set` accepts any path — the destination is the thing that actually
varies between callers, and cred has no opinion about where you keep things.

## What cred deliberately does not do

- **Run a command with the secret in its environment.** Use
  [`gopass env`](https://www.gopass.pw/) — it execs the child via `exec(3)`,
  so no parent process lingers holding the value in its own environment.
- **Encrypt secrets at rest in a repo.** Use [`sops`](https://github.com/getsops/sops)
  or [`dotenvx`](https://dotenvx.com/).
- **Be a password manager.** Use [`pass`](https://www.passwordstore.org/),
  [`gopass`](https://www.gopass.pw/), or the 1Password CLI.
- **Print a value to stdout.** There is deliberately no `cred get`.
  Printing a credential to stdout is exactly how it ends up in a shell
  history, a log, or an agent's transcript — the failure mode this tool
  exists to avoid, not a missing feature.

## Development

```bash
just build   # go build
just test    # go test ./...
just fmt     # gofmt -w .
just gate    # gofmt check + go vet + go test — the pre-push gate
```

## License

[MIT](LICENSE) © Lachlan Collins
