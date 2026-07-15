package box

type Worker = interface {
	Helper() int
	Stay() int
}

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use(w Worker) int { return w.Helper() + w.Stay() }
func UseBox(b Box) int { return b.Helper() + b.Stay() }
