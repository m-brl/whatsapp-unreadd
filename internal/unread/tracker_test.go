package unread

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func jid(user string) types.JID {
	return types.NewJID(user, types.DefaultUserServer)
}

func TestSeedAndTotal(t *testing.T) {
	tr := NewTracker()
	tr.Seed(jid("alice"), 3)
	tr.Seed(jid("bob"), 2)

	if total := tr.Total(); total != 5 {
		t.Fatalf("Total() = %d, want 5", total)
	}
}

func TestSeedZeroRemovesChat(t *testing.T) {
	tr := NewTracker()
	tr.Seed(jid("alice"), 3)
	tr.Seed(jid("alice"), 0)

	if total := tr.Total(); total != 0 {
		t.Fatalf("Total() = %d, want 0", total)
	}
}

func TestOnMessageIncrements(t *testing.T) {
	tr := NewTracker()
	tr.OnMessage(jid("alice"))
	tr.OnMessage(jid("alice"))
	tr.OnMessage(jid("bob"))

	if total := tr.Total(); total != 3 {
		t.Fatalf("Total() = %d, want 3", total)
	}
}

func TestOnReadClearsChat(t *testing.T) {
	tr := NewTracker()
	tr.Seed(jid("alice"), 5)
	tr.OnMessage(jid("bob"))
	tr.OnRead(jid("alice"))

	if total := tr.Total(); total != 1 {
		t.Fatalf("Total() = %d, want 1", total)
	}
}
