package demo

type Base struct{ N int }
type Other struct{ M int }
type Box struct {
	Base
	Other
}

func Use(b Box) int {
	return b.Base.N + b.Other.M
}
