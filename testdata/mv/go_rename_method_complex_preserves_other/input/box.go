package box

type Box struct{}
type Other struct{}

func (Box) Helper() int   { return 1 }
func (Box) Stay() int     { return 2 }
func (Other) Helper() int { return 9 }
func (Other) Stay() int   { return 8 }

func Use(v any, xs []Other) int {
	return v.(Box).Helper() + xs[0].Helper() + Box{}.Helper() + Other{}.Helper()
}
