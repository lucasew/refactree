package pkga

type Box struct{}

func (b *Box) Helper() int { return 1 }
func (b *Box) Stay() int   { return 2 }
