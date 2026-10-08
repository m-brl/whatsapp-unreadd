# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`whatsapp-unreadd` is a daemon that stays connected to WhatsApp over the
multi-device web API (via [whatsmeow](https://go.mau.fi/whatsmeow)) and
keeps an in-memory count of unread messages. It's meant to run as a
systemd **user** service on the developer's own machine.

Scope is deliberately narrow: track the unread count, expose it over a
local Unix socket (see `internal/ipc`), and fire a desktop notification
(sender + message) for each incoming message (see `internal/notify`). The
actual waybar module that reads from the socket is a future step and lives
outside this repo — don't build it here unless asked.

## Commands

This is a NixOS machine with no global Go toolchain; always run Go through
the flake dev shell (or `nix-shell -p go` if the flake env isn't set up):

```sh
nix develop --command go build ./...
nix develop --command go vet ./...
nix develop --command go test ./...
nix develop --command go test ./internal/unread/ -run TestOnReadClearsChat -v  # single test
nix develop --command gofmt -l .                                              # must print nothing
```

Build the daemon binary:

```sh
nix develop --command go build -o whatsapp-unreadd .
```

Pairing requires an interactive terminal (it prints a QR code), so run the
binary directly at least once before installing it as a service — see
README.md for the full pairing and systemd-install steps.

## Architecture

- `main.go` — process entry point. Opens the sqlite session store
  (`store/sqlstore`, pure-Go `modernc.org/sqlite` driver — no cgo), creates
  the whatsmeow client, does QR pairing if no session exists yet, and wires
  a single event handler (`makeEventHandler`) into `client.AddEventHandler`.
- `internal/unread.Tracker` — tracks per-chat unread counts. A
  mutex-guarded `map[types.JID]int`, with `Total()` summing across chats.
  Intentionally decoupled from whatsmeow types/events so it stays
  unit-testable without a live connection (see `tracker_test.go`).
- `internal/ipc.Server` — a Unix socket server. On every event that changes
  the tracked total, `main.go`'s `report()` closure calls both
  `log.Infof` and `srv.Broadcast(total)`, so the socket and the logs always
  agree. See the package doc comment in `internal/ipc/server.go` for the
  wire protocol (plain decimal + newline, push-only, no client requests).
- `internal/notify.Send` — thin wrapper around the `notify-send` binary
  (libnotify). `message.go`'s `messageText` turns the raw `*waE2E.Message`
  into a short summary (plain text, or an emoji placeholder for
  photos/videos/audio/stickers/etc.); `main.go`'s `notifyIncoming` supplies
  the title (`Info.PushName`, falling back to the sender's phone number)
  and fires the notification in a goroutine so a slow/missing notification
  daemon can't stall event processing.

### How the unread count is derived

whatsmeow does not expose a ready-made unread counter, so `main.go`
reconstructs one from three event types:

1. `*events.HistorySync` — WhatsApp sends a history blob after pairing (and
   sometimes later). Each `Conversation` in it carries an authoritative
   `UnreadCount` for that chat at that point in time. This seeds/overwrites
   `Tracker` state per chat (`Tracker.Seed`).
2. `*events.Message` — any incoming message (not `IsFromMe`) increments
   that chat's counter (`Tracker.OnMessage`).
3. `*events.Receipt` — when one of our *own* other devices (phone) reads a
   chat, WhatsApp reports it as a receipt where `IsFromMe` is true and
   `Type` is `types.ReceiptTypeRead` or `types.ReceiptTypeReadSelf`
   (`ReceiptTypeReadSelf` specifically covers the case where read receipts
   are disabled in privacy settings). Either case clears that chat's
   counter (`Tracker.OnRead`).

Broadcast lists and the `status@broadcast` JID are filtered out
(`isIgnoredChat`) since they aren't regular conversations.

When modifying this logic, keep in mind receipts/messages can arrive out of
order relative to the history sync blob — `Tracker` is written so that any
arrival order converges to the same total, and that property is what the
tests in `internal/unread/tracker_test.go` pin down.

## Dependencies worth knowing about

- `go.mau.fi/whatsmeow` — the WhatsApp client library. Its public API
  (event types, store interfaces) moves between pseudo-versions; when
  something doesn't compile after a `go get -u`, check the installed
  version's source directly under `$(go env GOMODCACHE)/go.mau.fi/whatsmeow@*`
  rather than trusting older examples found online.
- `modernc.org/sqlite` — pure-Go SQLite driver registered under driver name
  `"sqlite"` (not `"sqlite3"`, which is the cgo `mattn/go-sqlite3` driver
  used in whatsmeow's own README example). Keep using the pure-Go driver so
  the daemon cross-compiles without cgo. `main.go` opens it with WAL mode,
  a `busy_timeout` pragma, and `SetMaxOpenConns(1)` — without those,
  whatsmeow's concurrent session-store writes during decryption race with
  history sync writes and fail with `SQLITE_BUSY`.
- `github.com/mdp/qrterminal/v3` — renders the pairing QR code to the
  terminal.
