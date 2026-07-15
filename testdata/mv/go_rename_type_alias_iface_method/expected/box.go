package box

type Worker = interface {
	Assist() int
	Stay() int
}

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(w Worker) int { return w.Assist() + w.Stay() }
func UseBox(b Box) int { return b.Assist() + b.Stay() }
