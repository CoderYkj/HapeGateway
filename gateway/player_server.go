package gateway

import (
	"context"
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/CoderYkj/HapeGateway/link"
	"github.com/CoderYkj/HapeGateway/protocol/playerv1"
	"github.com/CoderYkj/HapeGateway/protocol/worldv1"
)

// Option customizes the player connection handler.
type Option func(*handlerConfig)

type handlerConfig struct{ metrics *Metrics }

// WithMetrics attaches a metric set to the handler.
func WithMetrics(m *Metrics) Option { return func(c *handlerConfig) { c.metrics = m } }

// NewPlayerConnHandler serves a player connection: hello/resume, request
// responses, and optional state pushes.
func NewPlayerConnHandler(backend Backend, auth Authenticator, opts ...Option) link.ConnHandler {
	if auth == nil {
		auth = DevAuthenticator{}
	}
	cfg := &handlerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	m := cfg.metrics
	return func(ctx context.Context, c *link.Conn) {
		if m != nil {
			m.ConnectionsTotal.Add(1)
			m.ConnectionsActive.Add(1)
			defer m.ConnectionsActive.Add(-1)
		}
		id, payload, err := c.Read()
		if err != nil {
			return
		}
		var hello playerv1.Hello
		if err := proto.Unmarshal(payload, &hello); err != nil {
			_ = replyHello(c, id, &playerv1.HelloAck{Error: "bad hello"})
			return
		}
		sess, err := auth.Authenticate(hello.Token)
		if err != nil {
			if m != nil {
				m.AuthFailures.Add(1)
			}
			_ = replyHello(c, id, &playerv1.HelloAck{Error: "unauthorized"})
			return
		}
		st, ok := backend.State(ctx, sess.PlayerID)
		version := uint64(0)
		if ok && st != nil {
			version = st.Version
		}
		ack := &playerv1.HelloAck{Ok: true, StateVersion: version}
		if ok && hello.LastStateVersion != version {
			if d, derr := loadDelta(ctx, backend, sess.PlayerID, hello.LastStateVersion); derr == nil {
				ack.Delta = d
			} else {
				ack.State = st
			}
		}
		if err := replyHello(c, id, ack); err != nil {
			return
		}
		lastVersion := version

		for {
			id, payload, err := c.Read()
			if err != nil {
				return
			}
			var in playerv1.Request
			if err := proto.Unmarshal(payload, &in); err != nil {
				_ = replyWorld(c, id, &worldv1.Response{Error: "bad request"})
				continue
			}
			if in.Token != "" {
				s, err := auth.Authenticate(in.Token)
				if err != nil || s.PlayerID != sess.PlayerID {
					if m != nil {
						m.AuthFailures.Add(1)
					}
					_ = replyWorld(c, id, &worldv1.Response{Error: "unauthorized"})
					continue
				}
			}
			cmd := in.GetCommand()
			if cmd == nil {
				_ = replyWorld(c, id, &worldv1.Response{Error: "missing command"})
				continue
			}
			if cmd.PlayerId == 0 {
				cmd.PlayerId = sess.PlayerID
			}
			if cmd.PlayerId != sess.PlayerID {
				_ = replyWorld(c, id, &worldv1.Response{Error: "player mismatch"})
				continue
			}
			if m != nil {
				m.CommandsTotal.Add(1)
			}
			resp, herr := backend.Handle(ctx, cmd)
			if herr != nil {
				resp = &worldv1.Response{Error: herr.Error()}
			}
			if resp == nil {
				resp = &worldv1.Response{}
			}
			if m != nil && (herr != nil || !resp.Ok) {
				m.CommandErrors.Add(1)
			}
			if err := replyWorld(c, id, resp); err != nil {
				return
			}
			if hello.Subscribe && resp.Ok && isMutation(cmd.Action) {
				if st, ok := backend.State(ctx, sess.PlayerID); ok && st != nil {
					ev := &playerv1.Event{StateVersion: st.Version}
					if d, derr := loadDelta(ctx, backend, sess.PlayerID, lastVersion); derr == nil {
						ev.Delta = d
					} else {
						ev.State = st
					}
					b, err := proto.Marshal(ev)
					if m != nil {
						m.PushesTotal.Add(1)
					}
					if err != nil || c.Push(b) != nil {
						return
					}
					lastVersion = st.Version
				}
			}
		}
	}
}

func loadDelta(ctx context.Context, backend Backend, pid, from uint64) (*worldv1.StateDelta, error) {
	resp, err := backend.Handle(ctx, &worldv1.Command{PlayerId: pid, Action: "state_delta", FromVersion: from})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.Ok {
		return nil, errors.New("state delta unavailable")
	}
	d := resp.GetDelta()
	if d == nil {
		return nil, errors.New("state delta unavailable")
	}
	return d, nil
}

func replyHello(c *link.Conn, id uint64, ack *playerv1.HelloAck) error {
	b, err := proto.Marshal(ack)
	if err != nil {
		return err
	}
	return c.Reply(id, b)
}

func replyWorld(c *link.Conn, id uint64, resp *worldv1.Response) error {
	b, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	return c.Reply(id, b)
}

func isMutation(action string) bool {
	switch action {
	case "state", "rank", "top", "trade_get", "battle_get", "battle_history", "state_delta":
		return false
	default:
		return true
	}
}
