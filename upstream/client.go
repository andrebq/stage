package upstream

import (
	"context"

	"github.com/andrebq/stage"
	"github.com/andrebq/stage/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type (
	client struct {
		c       protocol.UpstreamClient
		stageID string
	}
)

var (
	_ = stage.Upstream((*client)(nil))
)

func Dial(ctx context.Context, stageID, upstream string) (stage.Upstream, error) {
	conn, err := grpc.DialContext(ctx, upstream, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	cli := protocol.NewUpstreamClient(conn)
	return &client{c: cli, stageID: stageID}, nil
}

func (c *client) Close() error { return nil }
func (c *client) ID() string   { return c.stageID }
func (c *client) RegisterPIDs(ctx context.Context, pids ...stage.Identity) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "StageID", c.stageID)
	pidset := protocol.PIDSet{}
	for _, p := range pids {
		pidset.Pids = append(pidset.Pids, p.PID)
	}
	_, err := c.c.RegisterPIDs(ctx, &pidset)
	return err
}

func (c *client) Proxy(ctx context.Context, msgs ...stage.Message) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "StageID", c.stageID)
	pr := protocol.ProxyRequest{}
	for _, m := range msgs {
		pr.Messages = append(pr.Messages, &protocol.StageMessage{
			From:    m.From.PID,
			To:      m.To.PID,
			Method:  m.Method,
			Content: m.Content,
		})
	}
	_, err := c.c.Proxy(ctx, &pr)
	return err
}

func (c *client) Fetch(ctx context.Context) ([]stage.Message, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "StageID", c.stageID)
	var out []stage.Message
	fr, err := c.c.Fetch(ctx, &protocol.FetchRequest{})
	if err != nil {
		return nil, err
	}
	for _, sm := range fr.GetBatch() {
		out = append(out, stage.Message{
			From:    stage.Identity{PID: sm.GetFrom()},
			To:      stage.Identity{PID: sm.GetTo()},
			Method:  sm.GetMethod(),
			Content: sm.GetContent(),
		})
	}
	return out, nil
}
