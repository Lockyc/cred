# cred

[![Release](https://img.shields.io/github/v/release/lockyc/cred?sort=semver&label=release)](https://github.com/lockyc/cred/releases/latest)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-555)
![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)
[![License](https://img.shields.io/github/license/lockyc/cred)](LICENSE)

Put an API key or token into a file without it ever appearing in your shell
history, a process list or an agent's transcript. When an agent needs you to
place a credential, it hands you `cred set <path>`. You paste the secret at a
hidden prompt, and the agent gets back a receipt showing the file is right,
never the value. It works the same in bash, fish, zsh, or typed by hand.

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

The indented block is the receipt: hand it back to the agent that asked. It
shows the path, mode, size and a fingerprint, never the value. `--name KEY`
does the same for one key in a `.env` file and leaves the rest of the file
untouched.

Every flag, the mode and owner rules, `.env` quoting, and `show`/`rm`:
[docs/usage.md](docs/usage.md).

## What cred does not do

- **Run a command with the secret in its environment.** Use
  [`gopass env`](https://www.gopass.pw/).
- **Encrypt secrets at rest in a repo.** Use [`sops`](https://github.com/getsops/sops)
  or [`dotenvx`](https://dotenvx.com/).
- **Be a password manager.** Use [`pass`](https://www.passwordstore.org/),
  [`gopass`](https://www.gopass.pw/), or the 1Password CLI.
- **Print a value to stdout.** There is no `cred get`.

## Status

In use, `0.x`. What's next: [docs/roadmap.md](docs/roadmap.md).

## Development

```bash
just build   # go build
just test    # go test ./...
just fmt     # gofmt -w .
just gate    # gofmt check + go vet + go test
just install # go install, then runs the installed binary's `version`
```

Branch model and release process: [CLAUDE.md](CLAUDE.md).

## License

[MIT](LICENSE) © Lachlan Collins
