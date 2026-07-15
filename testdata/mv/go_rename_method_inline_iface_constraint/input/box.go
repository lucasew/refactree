package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Call[T interface{ Helper() int }](t T) int {
	return t.Helper()
}

func Use() int {
	return Call(Box{}) + Box{}.Stay()
}
