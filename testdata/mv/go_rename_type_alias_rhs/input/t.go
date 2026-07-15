package demo

type Box struct{ N int }
type Alias = Box

func Use(c Alias) int { return c.N }
func Make() Box       { return Box{N: 1} }
