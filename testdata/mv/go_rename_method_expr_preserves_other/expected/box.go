package box

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

type Other struct{}

func (Other) Helper() int { return 9 }

func Use() int {
	f := Box.Assist
	g := Other.Helper
	return f(Box{}) + g(Other{}) + Box{}.Stay()
}
