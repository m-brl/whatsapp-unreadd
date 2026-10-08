// Package unread maintains an in-memory count of unread WhatsApp messages
// per chat, kept up to date from whatsmeow events.
package unread

import (
	"sync"

	"go.mau.fi/whatsmeow/types"
)

// Tracker holds the unread message count for every chat the daemon knows
// about. It is safe for concurrent use.
type Tracker struct {
	mu     sync.Mutex
	counts map[types.JID]int
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{counts: make(map[types.JID]int)}
}

// Seed sets the unread count for a chat from a known-good source (the
// history sync WhatsApp sends on first login). It overwrites any previous
// value for that chat.
func (t *Tracker) Seed(chat types.JID, count int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if count <= 0 {
		delete(t.counts, chat)
		return
	}
	t.counts[chat] = count
}

// OnMessage records a newly received message in chat.
func (t *Tracker) OnMessage(chat types.JID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[chat]++
}

// OnRead clears the unread count for chat, as reported by a read receipt
// from one of our own devices.
func (t *Tracker) OnRead(chat types.JID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.counts, chat)
}

// Total returns the sum of unread counts across all tracked chats.
func (t *Tracker) Total() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := 0
	for _, c := range t.counts {
		total += c
	}
	return total
}
