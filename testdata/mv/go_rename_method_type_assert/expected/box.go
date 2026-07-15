package box

type Worker interface {
	Assist() int
	Stay() int
}

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(v any) int {
	return v.(Worker).Assist() + v.(Worker).Stay()
}
