package box

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(p *Box) int {
	return (*p).Assist() + p.Stay()
}
