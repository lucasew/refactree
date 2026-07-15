package demo

type Helper struct{}
type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Helper() int { return 9 }

func Use(b Box) int {
	return b.Helper() + b.Stay() + Helper()
}

func Make() Helper { return Helper{} }
