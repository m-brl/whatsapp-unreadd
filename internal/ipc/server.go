// Package ipc exposes the daemon's unread count to local clients (e.g. a
// waybar module) over a Unix domain socket.
//
// Protocol: on connect, the server immediately writes the current count as
// a decimal integer followed by a newline, then writes a new line every
// time the count changes. The connection is otherwise one-directional —
// clients don't need to send anything. This supports both a one-shot
// client that reads a single line and disconnects (e.g. waybar's
// interval-based "exec") and a long-lived client that keeps the connection
// open and updates on every line (e.g. waybar's streaming "exec", via
// something like `socat - UNIX-CONNECT:/path/to.sock`).
package ipc

import (
	"fmt"
	"net"
	"os"
	"sync"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Server broadcasts an integer value to every connected client.
type Server struct {
	log      waLog.Logger
	listener net.Listener

	mu      sync.Mutex
	clients map[net.Conn]struct{}
	current int
	closed  bool
}

// Listen creates a Unix socket at path, removing any stale socket file left
// over from a previous run, and starts accepting client connections.
func Listen(path string, log waLog.Logger) (*Server, error) {
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove stale socket: %w", err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on socket: %w", err)
	}
	s := &Server{
		log:      log,
		listener: listener,
		clients:  make(map[net.Conn]struct{}),
	}
	go s.acceptLoop()
	return s, nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.addClient(conn)
	}
}

func (s *Server) addClient(conn net.Conn) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		conn.Close()
		return
	}
	s.clients[conn] = struct{}{}
	current := s.current
	s.mu.Unlock()

	if err := writeLine(conn, current); err != nil {
		s.removeClient(conn)
		return
	}
	go s.detectDisconnect(conn)
}

// detectDisconnect blocks reading from conn purely to notice when the
// client goes away, since clients never send data in this protocol.
func (s *Server) detectDisconnect(conn net.Conn) {
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			s.removeClient(conn)
			return
		}
	}
}

func (s *Server) removeClient(conn net.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

// Broadcast updates the current value and pushes it to every connected
// client, dropping any client whose connection has gone away.
func (s *Server) Broadcast(count int) {
	s.mu.Lock()
	s.current = count
	conns := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	for _, conn := range conns {
		if err := writeLine(conn, count); err != nil {
			s.removeClient(conn)
		}
	}
}

// Close stops accepting new clients, disconnects existing ones, and removes
// the socket file.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	conns := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		conns = append(conns, c)
	}
	s.clients = make(map[net.Conn]struct{})
	s.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}
	return s.listener.Close()
}

func writeLine(conn net.Conn, count int) error {
	_, err := fmt.Fprintf(conn, "%d\n", count)
	return err
}
