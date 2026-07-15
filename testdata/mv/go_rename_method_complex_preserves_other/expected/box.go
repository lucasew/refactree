package box

type Box struct{}
type Other struct{}

func (Box) Assist() int   { return 1 }
func (Box) Stay() int     { return 2 }
func (Other) Helper() int { return 9 }
func (Other) Stay() int   { return 8 }

func Use(v any, xs []Other) int {
	return v.(Box).Assist() + xs[0].Helper() + Box{}.Assist() + Other{}.Helper()
}
