package demo

type Core struct{ N int }
type Other struct{ M int }
type Box struct {
	Core
	Other
}

func Use(b Box) int {
	return b.Core.N + b.Other.M
}
