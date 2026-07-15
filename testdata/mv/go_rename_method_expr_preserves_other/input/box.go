package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

type Other struct{}

func (Other) Helper() int { return 9 }

func Use() int {
	f := Box.Helper
	g := Other.Helper
	return f(Box{}) + g(Other{}) + Box{}.Stay()
}
