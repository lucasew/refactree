package box

type Box int

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(a int) int {
	return Box(a).Assist() + Box(a).Stay()
}
