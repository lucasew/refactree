package main

type Box struct {
	Helper int
	Stay   int
}

func Make() Box {
	return Box{Helper: 1, Stay: 2}
}

func Use(b Box) int {
	return b.Helper + b.Stay
}
