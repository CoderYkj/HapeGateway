package gateway

import (
	"errors"
	"fmt"
	"sync"
)

// Router selects the world backend for a session.
type Router interface {
	Backend(worldID uint32) (Backend, error)
}

// StaticRouter sends every session to one backend.
type StaticRouter struct{ B Backend }

func (s StaticRouter) Backend(uint32) (Backend, error) {
	if s.B == nil {
		return nil, errors.New("no backend")
	}
	return s.B, nil
}

// WorldMap routes world IDs to independent TCP backend pools.
type WorldMap struct {
	mu       sync.RWMutex
	backends map[uint32]*TCPBackend
	fallback *TCPBackend
}

// NewWorldMap builds one backend pool per world plus an optional fallback.
func NewWorldMap(addrByWorld map[uint32]string, fallbackAddr string, poolSize int) *WorldMap {
	m := &WorldMap{backends: make(map[uint32]*TCPBackend, len(addrByWorld))}
	for id, addr := range addrByWorld {
		m.backends[id] = NewTCPBackend(addr, poolSize)
	}
	if fallbackAddr != "" {
		m.fallback = NewTCPBackend(fallbackAddr, poolSize)
	}
	return m
}

// Backend returns the pool for worldID, or the fallback.
func (m *WorldMap) Backend(worldID uint32) (Backend, error) {
	m.mu.RLock()
	b := m.backends[worldID]
	m.mu.RUnlock()
	if b != nil {
		return b, nil
	}
	if m.fallback != nil {
		return m.fallback, nil
	}
	return nil, fmt.Errorf("no backend for world %d", worldID)
}

// PoolSizes reports each world's pool size.
func (m *WorldMap) PoolSizes() map[uint32]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[uint32]int, len(m.backends))
	for id, b := range m.backends {
		out[id] = b.Size()
	}
	return out
}

// SetSize changes one world's pool size.
func (m *WorldMap) SetSize(worldID uint32, n int) error {
	m.mu.RLock()
	b := m.backends[worldID]
	m.mu.RUnlock()
	if b == nil {
		if m.fallback == nil {
			return fmt.Errorf("no backend for world %d", worldID)
		}
		b = m.fallback
	}
	b.SetSize(n)
	return nil
}
