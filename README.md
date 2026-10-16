# cred

[![Release](https://img.shields.io/github/v/release/lockyc/cred?sort=semver&label=release)](https://github.com/lockyc/cred/releases/latest)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/lockyc/cred)](LICENSE)

A command that puts a credential into a file, without ever showing it to
anyone who didn't type it. When an agent needs you to place an API key or a
token, it hands you `cred set <path>`, which works the same in bash, fish,
zsh, or typed by hand.

## Install

```bash
go install github.com/lockyc/cred@latest
```

Or via the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/lockyc/cred/main/install.sh | bash
```

Both need Go on `PATH`; the script also prints where the binary landed.

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

**`set`** prompts on the controlling terminal (even when stdin is piped) and
writes the value to `<path>`. Each character you type or
paste is shown as one `●`, never as the character itself, so you can tell
your input registered without it ever appearing on screen. Backspace erases
one `●` per character removed; arrow keys and other escape
sequences are ignored.
`--value-from '<command>'` reads the value from another command's stdout
instead of prompting. Ctrl-C, or Ctrl-D on an empty prompt, aborts and
writes nothing.

**`--value-from` must name a command that *fetches* the value, never one
that *contains* it.** `--value-from 'op read op://vault/item/field'` keeps
the credential out of argv and shell history; `--value-from 'echo sk-live-abc...'`
puts it in both.

`--expect-prefix` refuses to write if the value doesn't start with the given
string, catching a wrong paste before it lands. `--mode` sets the octal mode
(default `600`). When cred writes the whole file, it always sets this mode,
even if the file already exists. With `--name` (one key inside a `.env` file,
below), an existing file keeps its mode; `--mode` applies only when cred
creates it.

Worked example — a standalone credential file:

```console
$ cred set ~/.config/example/token --expect-prefix tok_
Value for /home/you/.config/example/token: ●●●●●●●●●●●●●●●●●●●●●●●●●●
cred: OK
  path         /home/you/.config/example/token
  mode         600
  bytes        26
  fingerprint  97d90e6a6af2
  prefix       tok_ ✓
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

`show` on a path that doesn't exist reports `cred: MISSING — <path> does not
exist` on stdout and exits 1 — `rm` reports absence the same way, in the same
words, on the same stream.

`set`, `show`, and `rm` all refuse a destination that isn't a plain
file — a directory, or a symlink (even one pointing at a real credential).

**`--name KEY`** redirects `set`/`show`/`rm` at one key inside a `.env`-style
file instead of the whole file — the rest of the file (comments, ordering,
every other key) is left untouched:

```console
$ cred set ~/project/.env --name API_KEY --expect-prefix sk_
Value for API_KEY: ●●●●●●●●●●●●●●●●●●●●●●●●●●●●
cred: OK
  path         /home/you/project/.env
  key          API_KEY
  mode         600
  bytes        28
  fingerprint  46c0a14c6781
  prefix       sk_ ✓
```

`cred rm ~/project/.env --name API_KEY` removes just that line; on a key
that isn't set, it reports nothing was removed and exits 1 without touching
the file.

`set --name` refuses a value that contains a newline and writes nothing. Any other value
is quoted wherever a dotenv loader or a shell would misread it, so the file
is as safe to `. ./.env` as to load.

### The receipt

Every successful `set`/`show` prints a receipt. When an agent asked you for
the credential, this block is what you hand back to it:

```
cred: OK
  path         /home/you/.config/example/token
  mode         600
  bytes        26
  fingerprint  97d90e6a6af2
  prefix       tok_ ✓
```

The agent learns the path, file mode, byte count, and a 12-hex-character
fingerprint (an unsalted SHA-256 prefix): enough to confirm the write
landed, never the value itself.

### Duplicate keys in a `.env` file

If a key appears more than once in a `.env` file, `set --name` and
`show --name` both refuse with an error and change nothing. `rm --name`
deletes every occurrence of the key; re-add it clean with `set`.

### Where credentials go

A standalone credential conventionally lives at `~/.config/<service>/<name>`,
but `set` accepts any path.

## What cred does not do

- **Run a command with the secret in its environment.** Use
  [`gopass env`](https://www.gopass.pw/).
- **Encrypt secrets at rest in a repo.** Use [`sops`](https://github.com/getsops/sops)
  or [`dotenvx`](https://dotenvx.com/).
- **Be a password manager.** Use [`pass`](https://www.passwordstore.org/),
  [`gopass`](https://www.gopass.pw/), or the 1Password CLI.
- **Print a value to stdout.** There is no `cred get`.

## Development

```bash
just build   # go build
just test    # go test ./...
just fmt     # gofmt -w .
just gate    # gofmt check + go vet + go test — the pre-push gate
just install # go install, then runs the installed binary's `version`
```

## License

[MIT](LICENSE) © Lachlan Collins
