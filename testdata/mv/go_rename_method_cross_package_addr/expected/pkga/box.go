package pkga

type Box struct{}

func (b *Box) Assist() int { return 1 }
func (b *Box) Stay() int   { return 2 }
