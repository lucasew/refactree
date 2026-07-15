package box

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Call[T interface{ Assist() int }](t T) int {
	return t.Assist()
}

func Use() int {
	return Call(Box{}) + Box{}.Stay()
}
