package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/CoderYkj/HapeGateway/gateway"
	"github.com/CoderYkj/HapeGateway/link"
)

func main() {
	addr := flag.String("addr", ":9001", "player TCP listen address")
	adminAddr := flag.String("admin", ":8081", "admin HTTP listen address")
	worldTCP := flag.String("world-tcp", "127.0.0.1:9000", "world TCP endpoint")
	pool := flag.Int("tcp-pool", 4, "world TCP connections per gateway")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := gateway.NewMetrics()
	backend := gateway.NewTCPBackend(*worldTCP, *pool)
	srv, err := link.ListenPlayer(*addr, gateway.NewPlayerConnHandler(backend, gateway.DevAuthenticator{}, gateway.WithMetrics(metrics)))
	if err != nil {
		log.Fatalf("player tcp listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/admin/pool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			n, err := strconv.Atoi(r.URL.Query().Get("size"))
			if err != nil {
				http.Error(w, "bad size", http.StatusBadRequest)
				return
			}
			backend.SetSize(n)
		}
		_, _ = fmt.Fprintf(w, "pool=%d\n", backend.Size())
	})
	admin := &http.Server{Addr: *adminAddr, Handler: mux}

	errCh := make(chan error, 2)
	go func() {
		log.Printf("gateway tcp listening on %s -> world %s (pool=%d)", *addr, *worldTCP, *pool)
		if err := srv.Serve(ctx); err != nil && !errors.Is(err, link.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		log.Printf("gateway admin listening on %s", *adminAddr)
		if err := admin.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("gateway shutting down")
	case err := <-errCh:
		log.Printf("gateway error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = admin.Shutdown(shutdownCtx)
	log.Printf("gateway stopped")
}
