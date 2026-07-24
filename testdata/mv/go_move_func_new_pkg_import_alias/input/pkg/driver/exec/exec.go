package exec

import "context"

type Cmd struct{}

func MustRun(ctx context.Context, name string, args ...string) *Cmd { return &Cmd{} }

func (c *Cmd) Run() error { return nil }
