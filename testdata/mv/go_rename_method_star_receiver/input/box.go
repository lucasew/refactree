package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(p *Box) int {
	return (*p).Helper() + p.Stay()
}
