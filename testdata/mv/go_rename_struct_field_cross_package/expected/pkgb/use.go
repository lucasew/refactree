package pkgb

import "example.com/m/pkga"

func Make() pkga.Box {
	return pkga.Box{Assist: 1, Stay: 2}
}

func MakePtr() *pkga.Box {
	return &pkga.Box{Assist: 3, Stay: 4}
}

func Use(b pkga.Box) int {
	return b.Assist + b.Stay
}

func SumFromProvider(p pkga.Provider) (int, error) {
	boxes, err := p.Resolve()
	if err != nil {
		return 0, err
	}
	sum := 0
	for _, b := range boxes {
		sum += b.Assist + b.Stay
	}
	return sum, nil
}
