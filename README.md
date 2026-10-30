# HapeGateway

Reusable player TCP gateway for HapeServer.

## Packages

- `link`: length-prefixed frame protocol, multiplexed world client, player server (`Conn`/`PlayerServer`).
- `protocol/playerv1`: player wire (`Hello`, `HelloAck`, `Request`, `Event`).
- `protocol/worldv1`: backend contract (`Command`, typed `Response`, `StateDelta`).
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