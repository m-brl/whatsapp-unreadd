package main

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestMessageTextPlainConversation(t *testing.T) {
	msg := &waE2E.Message{Conversation: proto.String("salut !")}
	if got := messageText(msg); got != "salut !" {
		t.Fatalf("messageText() = %q, want %q", got, "salut !")
	}
}

func TestMessageTextExtendedText(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("réponse à un message")}}
	if got := messageText(msg); got != "réponse à un message" {
		t.Fatalf("messageText() = %q, want %q", got, "réponse à un message")
	}
}

func TestMessageTextImageWithCaption(t *testing.T) {
	msg := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("regarde ça")}}
	if got := messageText(msg); got != "regarde ça" {
		t.Fatalf("messageText() = %q, want %q", got, "regarde ça")
	}
}

func TestMessageTextImageWithoutCaptionFallsBack(t *testing.T) {
	msg := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	if got := messageText(msg); got != "📷 Photo" {
		t.Fatalf("messageText() = %q, want %q", got, "📷 Photo")
	}
}

func TestMessageTextVoiceNote(t *testing.T) {
	msg := &waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true)}}
	if got := messageText(msg); got != "🎤 Message vocal" {
		t.Fatalf("messageText() = %q, want %q", got, "🎤 Message vocal")
	}
}

func TestMessageTextUnknownTypeFallsBack(t *testing.T) {
	msg := &waE2E.Message{}
	if got := messageText(msg); got != "[message]" {
		t.Fatalf("messageText() = %q, want %q", got, "[message]")
	}
}
