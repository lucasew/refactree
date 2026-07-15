package example

// Helper is a type whose name collides with the method leaf on Box.
type Helper struct {
	N int
}

func (h Helper) Run() int { return h.N }

type Box struct{}

func (Box) Assist() int { return 1 }

func (Box) Stay() int { return 2 }

func UseHelper(h Helper) int {
	return h.Run() + h.N
}

func UseBox(b Box) int {
	return b.Assist() + b.Stay()
}
