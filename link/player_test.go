package link

import (
	"context"
	"testing"
	"time"
)

func TestPlayerServerShutdown(t *testing.T) {
	srv, err := ListenPlayer("127.0.0.1:0", func(_ context.Context, c *Conn) {
		_, _, _ = c.Read()
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(context.Background()) }()

	c, err := Dial(srv.Addr(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}
