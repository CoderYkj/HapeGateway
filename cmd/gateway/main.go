package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/CoderYkj/HapeGateway/gateway"
	"github.com/CoderYkj/HapeGateway/link"
)

func main() {
	addr := flag.String("addr", ":9001", "player TCP listen address")
	adminAddr := flag.String("admin", ":8081", "admin HTTP listen address")
	worldTCP := flag.String("world-tcp", "127.0.0.1:9000", "default world TCP endpoint")
	worldMap := flag.String("world-map", "", "world routing, e.g. 1=127.0.0.1:9000,2=127.0.0.1:9100")
	pool := flag.Int("tcp-pool", 4, "world TCP connections per gateway")
	authSecret := flag.String("auth-secret", "", "HMAC secret; empty uses dev tokens")
	flag.Parse()

	spec := parseWorldMap(*worldMap)
	router := gateway.NewWorldMap(spec, *worldTCP, *pool)
	var auth gateway.Authenticator = gateway.DevAuthenticator{}
	if *authSecret != "" {
		auth = gateway.HMACAuthenticator{Secret: []byte(*authSecret)}
	}

	metrics := gateway.NewMetrics()
	srv, err := link.ListenPlayer(*addr, gateway.NewRoutedPlayerConnHandler(router, auth, gateway.WithMetrics(metrics)))
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
			world, err := strconv.ParseUint(r.URL.Query().Get("world"), 10, 32)
			if err != nil {
				http.Error(w, "bad world", http.StatusBadRequest)
				return
			}
			n, err := strconv.Atoi(r.URL.Query().Get("size"))
			if err != nil {
				http.Error(w, "bad size", http.StatusBadRequest)
				return
			}
			if err := router.SetSize(uint32(world), n); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(router.PoolSizes())
	})
	admin := &http.Server{Addr: *adminAddr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		log.Printf("gateway tcp listening on %s -> default %s, worlds=%v (pool=%d)", *addr, *worldTCP, spec, *pool)
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

func parseWorldMap(spec string) map[uint32]string {
	out := make(map[uint32]string)
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			log.Printf("ignore bad world map entry %q", pair)
			continue
		}
		id, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
		if err != nil {
			log.Printf("ignore bad world id %q", parts[0])
			continue
		}
		out[uint32(id)] = strings.TrimSpace(parts[1])
	}
	return out
}
