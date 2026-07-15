package box

type Box struct{ N int }

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	return Box{N: 1}.Assist() + Box{N: 2}.Stay()
}
