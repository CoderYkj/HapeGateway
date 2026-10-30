package link

import (
	"bufio"
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// ErrServerClosed is returned by Serve after Shutdown.
var ErrServerClosed = errors.New("player server closed")

// Conn is one player connection with explicit reply/push writes.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer
	wmu  sync.Mutex
}

// Read waits for the next request frame.
func (c *Conn) Read() (uint64, []byte, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	typ, id, payload, err := readFrame(c.br)
	if err != nil {
		return 0, nil, err
	}
	if typ != frameRequest {
		return 0, nil, errors.New("expected request frame")
	}
	return id, payload, nil
}

// Reply sends a response for one request id.
func (c *Conn) Reply(id uint64, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return writeFrame(c.bw, frameReply, id, payload)
}

// Push sends a server-initiated event.
func (c *Conn) Push(payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return writeFrame(c.bw, framePush, 0, payload)
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.conn.Close() }

// ConnHandler serves one player connection.
type ConnHandler func(ctx context.Context, c *Conn)

// PlayerServer accepts player connections and hands them to a ConnHandler.
type PlayerServer struct {
	ln      net.Listener
	handler ConnHandler

	mu     sync.Mutex
	closed bool
	conns  map[*Conn]struct{}
	wg     sync.WaitGroup
}

// ListenPlayer starts a player TCP listener.
func ListenPlayer(addr string, h ConnHandler) (*PlayerServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &PlayerServer{ln: ln, handler: h, conns: make(map[*Conn]struct{})}, nil
}

// Addr reports the bound address.
func (s *PlayerServer) Addr() string { return s.ln.Addr().String() }

// Serve accepts until Shutdown closes the listener.
func (s *PlayerServer) Serve(ctx context.Context) error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return ErrServerClosed
			}
			return err
		}
		c := &Conn{conn: conn, br: bufio.NewReader(conn), bw: bufio.NewWriter(conn)}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = c.Close()
			continue
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.conns, c)
				s.mu.Unlock()
				_ = c.Close()
			}()
			s.handler(ctx, c)
		}()
	}
}

// Shutdown stops accepting, closes active player connections, and waits for
// their handlers to exit or ctx to expire.
func (s *PlayerServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	_ = s.ln.Close()
	conns := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	for _, c := range conns {
		_ = c.Close()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close is Shutdown with a background context.
func (s *PlayerServer) Close() error { return s.Shutdown(context.Background()) }
