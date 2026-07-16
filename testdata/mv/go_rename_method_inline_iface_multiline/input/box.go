package box

type Box struct{}

func (Box) Helper() int { return 1 }

func Call[T interface {
	Helper() int
	comparable
}](t T) int {
	return t.Helper()
}

func Use() int { return Call(Box{}) }
