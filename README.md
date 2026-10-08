# whatsapp-unreadd

A small daemon that stays connected to WhatsApp (over the multi-device web
API, via [whatsmeow](https://go.mau.fi/whatsmeow)) and keeps an in-memory
count of unread messages.

The count is available locally over a Unix socket (see "Reading the count"
below); wiring an actual waybar module up to it is a future step. It also
fires a desktop notification (sender + message) for each incoming message,
via `notify-send` — make sure that's installed and a notification daemon
is running in your session.

## Building

A Nix flake dev shell provides Go:

```sh
nix develop
go build -o whatsapp-unreadd .
```

## First run (pairing)

Run the binary directly in a terminal the first time. Since no session is
stored yet, it prints a QR code to scan from WhatsApp on your phone
(Settings > Linked devices > Link a device):

```sh
./whatsapp-unreadd
```

The session is persisted to a sqlite database (default:
`$XDG_CONFIG_HOME/whatsapp-unread/session.db`, override with `-db`). Once
paired, subsequent runs reconnect automatically without a QR code.

## Running as a systemd user service

```sh
mkdir -p ~/.local/bin ~/.config/systemd/user
cp whatsapp-unreadd ~/.local/bin/
cp systemd/whatsapp-unreadd.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now whatsapp-unreadd
journalctl --user -u whatsapp-unreadd -f
```

Pair once interactively before enabling the service, since the service has
no terminal to display a QR code on.

## Reading the count

The daemon listens on a Unix socket (default:
`$XDG_RUNTIME_DIR/whatsapp-unreadd.sock`, override with `-socket`). On
connect it immediately writes the current count as a line, then writes a
new line every time the count changes; it never reads anything from the
client.

One-shot read (e.g. for a polling waybar `custom` module with `interval`):

```sh
socat -T2 - UNIX-CONNECT:"$XDG_RUNTIME_DIR/whatsapp-unreadd.sock" | head -n1
```

Live updates (e.g. for a streaming waybar `custom` module, which treats
each line of a long-running `exec` as a new value — no `interval` needed):

```sh
socat - UNIX-CONNECT:"$XDG_RUNTIME_DIR/whatsapp-unreadd.sock"
```

## Flags

- `-db` — path to the session sqlite database.
- `-socket` — path to the unread-count Unix socket.
- `-log-level` — `ERROR`, `WARN`, `INFO` (default), or `DEBUG`.
