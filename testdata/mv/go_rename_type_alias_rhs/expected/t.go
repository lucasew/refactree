package demo

type Crate struct{ N int }
type Alias = Crate

func Use(c Alias) int { return c.N }
func Make() Crate       { return Crate{N: 1} }
