package box

type Worker interface {
	Helper() int
	Stay() int
}

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(v any) int {
	return v.(Worker).Helper() + v.(Worker).Stay()
}
