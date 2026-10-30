package gateway

import (
	"context"

	"github.com/CoderYkj/HapeProto/worldv1"
)

// Backend is the world command surface behind the gateway.
type Backend interface {
	Handle(ctx context.Context, cmd *worldv1.Command) (*worldv1.Response, error)
	State(ctx context.Context, playerID uint64) (*worldv1.PlayerState, bool)
}
