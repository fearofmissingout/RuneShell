package netplay

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"cmdcards/internal/content"
)

type Snapshot = roomSnapshot
type Command = commandPayload

type Session struct {
	conn     net.Conn
	enc      *json.Encoder
	hostName string
	hostSrv  *server

	mu      sync.RWMutex
	current *Snapshot
	closed  bool

	snapshots chan *Snapshot
	errs      chan error
}

func StartHostedSession(lib *content.Library, port int, name, classID string, forceNew bool) (*Session, error) {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	savePath, err := defaultRoomSavePath()
	if err != nil {
		return nil, err
	}
	if forceNew {
		if err := clearSavedRoom(savePath); err != nil {
			return nil, err
		}
	}
	srv, restored, err := loadServerFromSavePath(lib, addr, savePath)
	if err != nil {
		return nil, err
	}
	if restored {
		srv.restoredFromSave = true
		if hostPlayer, ok := srv.players[srv.hostID]; ok && !strings.EqualFold(hostPlayer.Name, name) {
			_ = srv.listener.Close()
			return nil, fmt.Errorf("saved room belongs to host %q; use the same name or disable room restore", hostPlayer.Name)
		}
		srv.roomLog = append(srv.roomLog, "Saved room restored. Waiting for players to reconnect.")
	}
	_ = srv.persistLocked()
	go srv.serve()
	session, err := startJoinedSessionWithRetry(fmt.Sprintf("127.0.0.1:%d", port), name, classID, 3*time.Second)
	if err != nil {
		srv.mu.Lock()
		srv.shutdownLocked(fmt.Sprintf("%s failed to enter the host session. Room saved for restore.", name))
		srv.mu.Unlock()
		return nil, err
	}
	session.hostSrv = srv
	session.hostName = name
	return session, nil
}

func StartJoinedSession(addr, name, classID string) (*Session, error) {
	return startJoinedSession(addr, name, classID)
}

func startJoinedSessionWithRetry(addr, name, classID string, timeout time.Duration) (*Session, error) {
	conn, err := dialTCPWithRetry(addr, timeout)
	if err != nil {
		return nil, err
	}
	return startJoinedSessionOnConn(conn, name, classID)
}

func startJoinedSession(addr, name, classID string) (*Session, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return startJoinedSessionOnConn(conn, name, classID)
}

func startJoinedSessionOnConn(conn net.Conn, name, classID string) (*Session, error) {
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(message{Type: "hello", Hello: &helloPayload{Name: name, ClassID: classID}}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	session := &Session{
		conn:      conn,
		enc:       enc,
		snapshots: make(chan *Snapshot, 16),
		errs:      make(chan error, 2),
	}
	go session.readLoop(dec)
	return session, nil
}

func (s *Session) readLoop(dec *json.Decoder) {
	for {
		var msg message
		if err := dec.Decode(&msg); err != nil {
			s.errs <- err
			return
		}
		if msg.Type == "snapshot" && msg.Snapshot != nil {
			s.mu.Lock()
			s.current = msg.Snapshot
			s.mu.Unlock()
			s.snapshots <- msg.Snapshot
			continue
		}
		if msg.Type == "error" {
			s.errs <- fmt.Errorf("%s", msg.Error)
			return
		}
	}
}

func (s *Session) Snapshots() <-chan *Snapshot { return s.snapshots }
func (s *Session) Errors() <-chan error        { return s.errs }

func (s *Session) CurrentSnapshot() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Session) Send(cmd *Command) error {
	if cmd == nil {
		return nil
	}
	return s.enc.Encode(message{Type: "command", Command: cmd})
}

func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.hostSrv != nil {
		s.hostSrv.mu.Lock()
		s.hostSrv.shutdownLocked(fmt.Sprintf("%s ended the host session. Room saved for restore.", s.hostName))
		s.hostSrv.mu.Unlock()
	}
	return nil
}

func ParseTextCommand(snapshot *Snapshot, line string) (*Command, bool, error) {
	if snapshot == nil {
		return nil, false, fmt.Errorf("room snapshot is not ready yet")
	}
	return parseClientCommand(snapshot, line)
}

func IsGracefulRoomClose(err error) bool {
	return isGracefulRoomClose(err)
}
