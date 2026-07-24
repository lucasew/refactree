package exec

import "context"

type Cmd struct{}

func Run(ctx context.Context, name string, args ...string) (*Cmd, error) {
	return &Cmd{}, nil
}

func (c *Cmd) Run() error { return nil }
