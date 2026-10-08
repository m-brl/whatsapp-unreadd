// Package notify shows desktop notifications via notify-send (libnotify).
package notify

import (
	"os/exec"
	"strings"
)

const maxBodyRunes = 300

// Send shows a desktop notification with the given title and body. It
// requires notify-send on PATH and a running notification daemon (both
// normally present in a graphical session), and does not block on the
// notification being displayed any longer than notify-send itself does.
func Send(title, body string) error {
	cmd := exec.Command(
		"notify-send",
		"--app-name=whatsapp-unreadd",
		"--icon=mail-unread",
		sanitize(title),
		sanitize(truncate(body, maxBodyRunes)),
	)
	return cmd.Run()
}

func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// sanitize escapes characters notify-send's body-markup support might
// interpret as (X)HTML, so a stray "<" or "&" in a message can't swallow
// text or malform the popup.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
