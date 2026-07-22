package pkgb

import "example.com/m/pkga"

func FromMap(m map[string]pkga.Box) int {
	b := m["k"]
	return b.Helper + b.Stay
}

func FromMapPtr(m map[string]*pkga.Box) int {
	b := m["k"]
	return b.Helper + b.Stay
}

func FromSlice(xs []pkga.Box) int {
	b := xs[0]
	return b.Helper + b.Stay
}

func IndexDirect(xs []pkga.Box) int {
	return xs[0].Helper + xs[0].Stay
}

func MapIndexDirect(m map[string]pkga.Box) int {
	return m["k"].Helper + m["k"].Stay
}

func RangeMap(m map[string]pkga.Box) int {
	sum := 0
	for _, b := range m {
		sum += b.Helper + b.Stay
	}
	return sum
}

func RangeSlice(xs []pkga.Box) int {
	sum := 0
	for _, b := range xs {
		sum += b.Helper + b.Stay
	}
	return sum
}
