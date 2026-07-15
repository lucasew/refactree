package demo

type Helper struct{}
type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Helper() int { return 9 }

func Use(b Box) int {
	return b.Assist() + b.Stay() + Helper()
}

func Make() Helper { return Helper{} }
