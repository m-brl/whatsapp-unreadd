package main

import (
	"fmt"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// messageText returns a short, human-readable summary of msg, suitable for
// a notification body. Message types without plain text (stickers, calls,
// etc.) get a placeholder instead of being silently dropped.
func messageText(msg *waE2E.Message) string {
	switch {
	case msg.GetConversation() != "":
		return msg.GetConversation()
	case msg.GetExtendedTextMessage().GetText() != "":
		return msg.GetExtendedTextMessage().GetText()
	case msg.GetImageMessage() != nil:
		return withFallback(msg.GetImageMessage().GetCaption(), "📷 Photo")
	case msg.GetVideoMessage() != nil:
		if msg.GetVideoMessage().GetGifPlayback() {
			return withFallback(msg.GetVideoMessage().GetCaption(), "🎞️ GIF")
		}
		return withFallback(msg.GetVideoMessage().GetCaption(), "🎥 Vidéo")
	case msg.GetAudioMessage() != nil:
		if msg.GetAudioMessage().GetPTT() {
			return "🎤 Message vocal"
		}
		return "🎵 Audio"
	case msg.GetDocumentMessage() != nil:
		doc := msg.GetDocumentMessage()
		if doc.GetCaption() != "" {
			return doc.GetCaption()
		}
		return withFallback(doc.GetFileName(), "📄 Document")
	case msg.GetStickerMessage() != nil:
		return "🏷️ Sticker"
	case msg.GetContactMessage() != nil:
		return fmt.Sprintf("👤 Contact : %s", msg.GetContactMessage().GetDisplayName())
	case msg.GetLocationMessage() != nil:
		if name := msg.GetLocationMessage().GetName(); name != "" {
			return fmt.Sprintf("📍 %s", name)
		}
		return "📍 Position"
	case msg.GetReactionMessage() != nil:
		return fmt.Sprintf("a réagi avec %s", msg.GetReactionMessage().GetText())
	default:
		return "[message]"
	}
}

func withFallback(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
