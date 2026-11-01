package gateway

import "testing"

func TestWorldMapRouting(t *testing.T) {
	m := NewWorldMap(map[uint32]string{1: "127.0.0.1:1", 2: "127.0.0.1:2"}, "127.0.0.1:3", 2)
	b1, err := m.Backend(1)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := m.Backend(2)
	fb, _ := m.Backend(9)
	if b1 == b2 || b1 == fb || b2 == fb {
		t.Fatal("world backends are not distinct")
	}
	if err := m.SetSize(2, 4); err != nil {
		t.Fatal(err)
	}
	if got := m.PoolSizes()[2]; got != 4 {
		t.Fatalf("world2 pool = %d", got)
	}
	if _, err := NewWorldMap(nil, "", 1).Backend(1); err == nil {
		t.Fatal("missing backend must error")
	}
}
