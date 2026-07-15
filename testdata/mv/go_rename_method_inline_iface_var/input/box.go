package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	var t interface{ Helper() int } = Box{}
	return t.Helper() + Box{}.Stay()
}
