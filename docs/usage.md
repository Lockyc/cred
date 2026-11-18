---
type: reference
links:
  - rel: part-of
    to: README.md
    note: README carries the synopsis and one example; this is every flag and rule
---

# cred usage reference

```bash
cred set  <path> [--name KEY] [--expect-prefix P] [--mode M] [--value-from CMD]
cred show <path> [--name KEY]
cred rm   <path> [--name KEY]
cred version
```

Flags may appear before or after the path (`cred set --name KEY <path>` and
`cred set <path> --name KEY` both work). Run `cred <command> -h` for a
command's own option list.

## Flags

| Flag | Default | What it does |
|---|---|---|
| `--name KEY` | none | Act on one key inside a `.env`-style file instead of the whole file (below). |
| `--expect-prefix P` | none | Refuse to write a value that doesn't start with `P`, catching a wrong paste before it lands. |
| `--mode M` | `600` | The file's permission bits, octal `000`–`777`. |
| `--value-from CMD` | none | Read the value from `CMD`'s stdout instead of prompting. |

## `set`

`set` prompts on the controlling terminal (even when stdin is piped) and
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

## Mode and owner

Writing a whole file always sets `--mode`, even on an existing file. With
`--name`, an existing file keeps its mode, and `--mode` applies only when cred
creates the file.

Rewriting an existing file keeps its owner and group, so `sudo cred set` on a
service's file leaves it readable by that service. If cred can't keep them —
you aren't root and the file belongs to someone else — it writes nothing.

## `show` and `rm`

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

## One key in a `.env` file: `--name`

`--name KEY` redirects `set`/`show`/`rm` at one key inside a `.env`-style
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

`set --name` quotes a value wherever a loader or a shell would misread it,
so godotenv, python-dotenv and `. ./.env` all read back exactly the value
typed. Three kinds of value have no such quoting, and `set --name` refuses
them and writes nothing: one containing a newline, one that is not valid
UTF-8, and one containing `'` or `\`
together with `$`, a backtick, or a trailing `\` or `"`.

If a key appears more than once in a `.env` file, `set --name` and
`show --name` both refuse with an error and change nothing. `rm --name`
deletes every occurrence of the key; re-add it clean with `set`.

## The receipt

Every successful `set`/`show` prints a receipt — the indented block in the
examples. When an agent asked you for the credential, that block
is what you hand back to it. The agent learns the path, file mode, byte count, and a 12-hex-character
fingerprint (an unsalted SHA-256 prefix): enough to confirm the write
landed, never the value itself.

## Where credentials go

A standalone credential conventionally lives at `~/.config/<service>/<name>`,
but `set` accepts any path.
