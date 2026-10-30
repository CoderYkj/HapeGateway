# HapeGateway

Go module: `github.com/CoderYkj/HapeGateway`

Reusable player TCP gateway for HapeServer.

## Packages

- `link`: length-prefixed frame protocol, multiplexed world client, player server (`Conn`/`PlayerServer`).
- `gateway`: authenticator, backend interface, player connection handler, pooled TCP backend.
- `cmd/gateway`: runnable gateway.

## Backend contract

```go
type Backend interface {
    Handle(ctx context.Context, cmd *worldv1.Command) (*worldv1.Response, error)
    State(ctx context.Context, playerID uint64) (*worldv1.PlayerState, bool)
}
```

`gateway.TCPBackend` implements it over a pooled TCP link to a world process.

The wire contracts come from `github.com/CoderYkj/HapeProto` (`playerv1`, `worldv1`).

## Run

```bash
go run ./cmd/gateway -addr :9001 -admin :8081 -world-tcp 127.0.0.1:9000 -tcp-pool 4
```

Player protocol: TCP frame payload is protobuf `player.v1.Hello` / `Request`;
server replies with `worldv1.Response` and optional `player.v1.Event` pushes.

## Test

```bash
go test ./...
```

## Metrics

`GET /metrics` on the admin port exposes Prometheus text:

```text
hapegateway_connections_total
hapegateway_connections_active
hapegateway_auth_failures_total
hapegateway_commands_total
hapegateway_command_errors_total
hapegateway_pushes_total
hapegateway_backend_errors_total
```

## Graceful shutdown

On SIGINT/SIGTERM the gateway stops accepting, closes active player
connections, waits up to 5s for handlers, then shuts down the admin HTTP
server.