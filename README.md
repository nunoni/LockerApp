# Locker

A local, terminal-based password and API key manager built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Security model

- The **master password** is hashed with Argon2id (64 MiB, 3 iterations) and
  only the hash is stored.
- A separate Argon2id-derived key encrypts every secret with
  **AES-256-GCM** (random nonce per entry). Secrets are never stored in
  plaintext.
- Only the secret is encrypted; entry names, groups, usernames and notes are
  stored in plaintext, so keep anything sensitive in the secret field.
- The vault lives at `~/.config/locker/vault.json` with `0600` permissions.
- The vault locks (key zeroed) on quit and automatically after 5 minutes of
  inactivity. SIGINT/SIGTERM/SIGHUP trigger the same clean shutdown.
- Writes are atomic (unique temp file, fsync, rename) and guarded by a
  per-write lock so concurrent processes cannot corrupt the vault.
- Copied secrets are cleared from the clipboard after 15s, but only if the
  clipboard still holds that secret; on KDE, the Klipper history is purged
  too. Quitting with a pending clear wipes the clipboard immediately.

## Requirements

- Go 1.26 or newer (see `go.mod`).
- `wl-clipboard`, `xclip`, or `xsel` for the copy-to-clipboard feature
  (copying fails gracefully without one).

## Run

```sh
go build -o locker .
./locker
```

First launch asks you to create a master password; after that, it asks you
to unlock.

## Keys

| Screen | Keys |
| --- | --- |
| List | `tab`/`→`/`l` or `shift+tab`/`←`/`h` switch Passwords/API Keys, `j`/`k` move, `enter` view, `a` add, `e` edit, `d` delete, `q` quit |
| Detail | `r` reveal secret, `c` copy (clears after 15s), `e` edit, `d` delete, `esc` back |
| Form | `tab` next field, `enter`/`ctrl+s` save, `esc` cancel |

Entries are grouped by their **group** field under each tab; leave the group
empty to use `General`. When editing, leave the secret field empty to keep
the current secret.

## CLI

Add entries without opening the TUI. The master password and the secret are
always read from stdin (one per line) when piped, or prompted for with echo
disabled on a terminal — never from command-line flags, so they never land in
argv or shell history:

```sh
locker add --type apikey --name "Stripe" --group "Payments"          # prompts hidden for master password and secret
printf '%s\n%s\n' "$MASTER_PW" "$SECRET" | locker add --name "T"     # piping: password then secret, one per line
```

Flags: `--type` (password|apikey, default apikey), `--name`, `--group`,
`--username`, `--notes`, `--vault` (custom vault path).

## Test

```sh
go test ./...
```
