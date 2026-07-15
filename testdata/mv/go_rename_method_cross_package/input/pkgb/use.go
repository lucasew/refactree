package pkgb

import "example.com/m/pkga"

func Use() int {
	b := pkga.Box{}
	return b.Helper() + b.Stay()
}
