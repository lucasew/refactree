package box

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	var t interface{ Assist() int } = Box{}
	return t.Assist() + Box{}.Stay()
}
