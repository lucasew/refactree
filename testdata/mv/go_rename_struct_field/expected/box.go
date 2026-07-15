package main

type Box struct {
	Assist int
	Stay   int
}

func Make() Box {
	return Box{Assist: 1, Stay: 2}
}

func Use(b Box) int {
	return b.Assist + b.Stay
}
