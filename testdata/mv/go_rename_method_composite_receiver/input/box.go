package box

type Box struct{ N int }

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	return Box{N: 1}.Helper() + Box{N: 2}.Stay()
}
