package box

type Box struct{}

func (b *Box) Helper() int { return 1 }
func (b *Box) Stay() int   { return 2 }

func Use() int {
	f := (*Box).Helper
	return f(&Box{}) + (*Box).Stay(&Box{})
}
