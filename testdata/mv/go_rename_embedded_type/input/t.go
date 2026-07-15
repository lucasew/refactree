package demo

type Base struct{ N int }
type Box struct{ Base }

func (b Box) Get() int { return b.N }
