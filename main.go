// Command whatsapp-unreadd is a small daemon that stays connected to
// WhatsApp (via whatsmeow's multi-device web API) and keeps track of how
// many messages are currently unread. It is meant to run under systemd;
// exposing the count to other tools (e.g. a waybar module) is left for a
// later iteration.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "modernc.org/sqlite"

	"whatsapp-unread/internal/ipc"
	"whatsapp-unread/internal/notify"
	"whatsapp-unread/internal/unread"
)

func main() {
	dbPath := flag.String("db", defaultDBPath(), "path to the session database (sqlite)")
	socketPath := flag.String("socket", defaultSocketPath(), "path to the unread-count Unix socket")
	logLevel := flag.String("log-level", "INFO", "log level: ERROR, WARN, INFO, DEBUG")
	flag.Parse()

	dbLog := waLog.Stdout("Database", *logLevel, true)
	log := waLog.Stdout("whatsapp-unreadd", *logLevel, true)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		log.Errorf("Failed to create database directory: %v", err)
		os.Exit(1)
	}

	// A single connection in WAL mode with a busy timeout avoids SQLITE_BUSY
	// errors: whatsmeow saves Signal sessions from several goroutines while
	// decrypting incoming messages, often while a history sync is also
	// writing, and SQLite only ever allows one writer at a time anyway.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", *dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Errorf("Failed to open session database: %v", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(1)

	container := sqlstore.NewWithDB(db, "sqlite", dbLog)
	if err := container.Upgrade(ctx); err != nil {
		log.Errorf("Failed to upgrade session store: %v", err)
		os.Exit(1)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Errorf("Failed to load device: %v", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*socketPath), 0o700); err != nil {
		log.Errorf("Failed to create socket directory: %v", err)
		os.Exit(1)
	}
	srv, err := ipc.Listen(*socketPath, log.Sub("IPC"))
	if err != nil {
		log.Errorf("Failed to start IPC socket: %v", err)
		os.Exit(1)
	}
	defer srv.Close()

	client := whatsmeow.NewClient(device, waLog.Stdout("Client", *logLevel, true))
	tracker := unread.NewTracker()
	client.AddEventHandler(makeEventHandler(tracker, srv, log))

	if client.Store.ID == nil {
		if err := pairWithQRCode(ctx, client, log); err != nil {
			log.Errorf("Login failed: %v", err)
			os.Exit(1)
		}
	} else if err := client.Connect(); err != nil {
		log.Errorf("Failed to connect: %v", err)
		os.Exit(1)
	}

	log.Infof("Daemon started, listening on %s", *socketPath)
	<-ctx.Done()
	log.Infof("Shutting down")
	client.Disconnect()
}

func defaultDBPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "whatsapp-unread", "session.db")
	}
	return "session.db"
}

func defaultSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "whatsapp-unreadd.sock")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "whatsapp-unread", "whatsapp-unreadd.sock")
	}
	return "whatsapp-unreadd.sock"
}

// pairWithQRCode connects the client and, since no session exists yet,
// prints a QR code to the terminal for the user to scan with their phone.
func pairWithQRCode(ctx context.Context, client *whatsmeow.Client, log waLog.Logger) error {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return err
	}
	if err := client.Connect(); err != nil {
		return err
	}
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			log.Infof("Scan the QR code above with WhatsApp (Linked devices)")
		case "success":
			log.Infof("Pairing successful")
		default:
			log.Infof("Login event: %s", evt.Event)
		}
	}
	return nil
}

// makeEventHandler wires whatsmeow events into the unread tracker, logging
// and broadcasting (over the IPC socket) the running total whenever it
// changes.
func makeEventHandler(tracker *unread.Tracker, srv *ipc.Server, log waLog.Logger) func(evt any) {
	report := func(context string) {
		total := tracker.Total()
		log.Infof("Unread count: %d%s", total, context)
		srv.Broadcast(total)
	}

	return func(evt any) {
		switch v := evt.(type) {
		case *events.HistorySync:
			for _, conv := range v.Data.GetConversations() {
				chat, err := types.ParseJID(conv.GetID())
				if err != nil || isIgnoredChat(chat) {
					continue
				}
				tracker.Seed(chat, int(conv.GetUnreadCount()))
			}
			report(" (history sync)")

		case *events.Message:
			if v.Info.IsFromMe || isIgnoredChat(v.Info.Chat) {
				return
			}
			tracker.OnMessage(v.Info.Chat)
			report("")
			notifyIncoming(v, log)

		case *events.Receipt:
			if !v.IsFromMe || isIgnoredChat(v.Chat) {
				return
			}
			if v.Type == types.ReceiptTypeRead || v.Type == types.ReceiptTypeReadSelf {
				tracker.OnRead(v.Chat)
				report("")
			}

		case *events.LoggedOut:
			log.Warnf("Logged out from WhatsApp, reason: %s", v.Reason)
		}
	}
}

// notifyIncoming shows a desktop notification for a newly received message,
// titled with the sender's name and bodied with a short summary of the
// message. It runs notify-send in the background so a slow or missing
// notification daemon never blocks event processing.
func notifyIncoming(evt *events.Message, log waLog.Logger) {
	sender := evt.Info.PushName
	if sender == "" {
		sender = evt.Info.Sender.User
	}
	body := messageText(evt.Message)

	go func() {
		if err := notify.Send(sender, body); err != nil {
			log.Warnf("Failed to show desktop notification: %v", err)
		}
	}()
}

// isIgnoredChat reports whether chat is a broadcast list or the status
// updates feed, neither of which count as a regular unread conversation.
func isIgnoredChat(chat types.JID) bool {
	return chat.IsBroadcastList() || chat == types.StatusBroadcastJID
}
