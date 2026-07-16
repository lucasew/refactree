package pkga

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

type Other struct{}

func (Other) Helper() int { return 9 }
