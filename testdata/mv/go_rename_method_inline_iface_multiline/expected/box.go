package box

type Box struct{}

func (Box) Assist() int { return 1 }

func Call[T interface {
	Assist() int
	comparable
}](t T) int {
	return t.Assist()
}

func Use() int { return Call(Box{}) }
