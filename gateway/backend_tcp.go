package gateway

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"hapegateway/link"
	"hapegateway/protocol/worldv1"
)

// TCPBackend forwards commands over a dynamically sized pool of TCP links.
// A player is consistently mapped to one slot so its commands keep order at
// the gateway hop.
type TCPBackend struct {
	addr string

	mu    sync.RWMutex
	slots []*tcpSlot
}

type tcpSlot struct {
	mu   sync.Mutex
	conn *link.Client
}

// NewTCPBackend builds a backend pool.
func NewTCPBackend(addr string, poolSize int) *TCPBackend {
	if poolSize <= 0 {
		poolSize = 4
	}
	r := &TCPBackend{addr: addr, slots: make([]*tcpSlot, poolSize)}
	for i := range r.slots {
		r.slots[i] = &tcpSlot{}
	}
	return r
}

// Size reports the current pool size.
func (r *TCPBackend) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.slots)
}

// SetSize grows or shrinks the pool at runtime.
func (r *TCPBackend) SetSize(n int) {
	if n < 1 {
		n = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.slots) < n {
		r.slots = append(r.slots, &tcpSlot{})
	}
	for len(r.slots) > n {
		last := r.slots[len(r.slots)-1]
		last.mu.Lock()
		if last.conn != nil {
			_ = last.conn.Close()
			last.conn = nil
		}
		last.mu.Unlock()
		r.slots = r.slots[:len(r.slots)-1]
	}
}

func (r *TCPBackend) pick(pid uint64) *tcpSlot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.slots) == 0 {
		return nil
	}
	idx := int((pid * 2654435761) % uint64(len(r.slots)))
	return r.slots[idx]
}

func (r *TCPBackend) client(s *tcpSlot) (*link.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return s.conn, nil
	}
	c, err := link.Dial(r.addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	s.conn = c
	return c, nil
}

func (r *TCPBackend) drop(s *tcpSlot, c *link.Client) {
	s.mu.Lock()
	if s.conn == c {
		s.conn = nil
	}
	s.mu.Unlock()
	_ = c.Close()
}

// Handle sends one command to the world.
func (r *TCPBackend) Handle(ctx context.Context, cmd *worldv1.Command) (*worldv1.Response, error) {
	slot := r.pick(cmd.PlayerId)
	if slot == nil {
		return nil, errors.New("tcp pool is empty")
	}
	c, err := r.client(slot)
	if err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	data, err := c.Call(ctx, payload)
	if err != nil {
		r.drop(slot, c)
		return nil, err
	}
	var resp worldv1.Response
	if err := proto.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// State loads a player's current state.
func (r *TCPBackend) State(ctx context.Context, playerID uint64) (*worldv1.PlayerState, bool) {
	resp, err := r.Handle(ctx, &worldv1.Command{PlayerId: playerID, Action: "state"})
	if err != nil || resp == nil || !resp.Ok {
		return nil, false
	}
	return resp.GetState(), true
}
