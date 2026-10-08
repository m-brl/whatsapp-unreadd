package ipc

import (
	"bufio"
	"net"
	"path/filepath"
	"testing"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func readLine(t *testing.T, conn net.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read line: %v", err)
	}
	return line
}

func TestClientReceivesCurrentValueOnConnect(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv, err := Listen(sockPath, waLog.Noop)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	srv.Broadcast(7)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	defer conn.Close()

	if line := readLine(t, conn); line != "7\n" {
		t.Fatalf("got %q, want %q", line, "7\n")
	}
}

func TestBroadcastPushesUpdateToConnectedClients(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv, err := Listen(sockPath, waLog.Noop)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if line, err := reader.ReadString('\n'); err != nil || line != "0\n" {
		t.Fatalf("initial line = %q, %v; want %q, nil", line, err, "0\n")
	}

	srv.Broadcast(3)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if line, err := reader.ReadString('\n'); err != nil || line != "3\n" {
		t.Fatalf("second line = %q, %v; want %q, nil", line, err, "3\n")
	}
}

func TestCloseDisconnectsClients(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv, err := Listen(sockPath, waLog.Noop)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	defer conn.Close()
	// Drain the initial value so Close()'s disconnect is unambiguous.
	readLine(t, conn)

	if err := srv.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatalf("expected connection to be closed after Close()")
	}
}
