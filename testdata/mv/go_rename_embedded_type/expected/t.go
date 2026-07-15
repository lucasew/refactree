package demo

type Core struct{ N int }
type Box struct{ Core }

func (b Box) Get() int { return b.N }
