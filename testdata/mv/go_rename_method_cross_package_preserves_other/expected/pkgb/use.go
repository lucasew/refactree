package pkgb

import "example.com/m/pkga"

func Use() int {
	b := pkga.Box{}
	o := pkga.Other{}
	return b.Assist() + o.Helper() + b.Stay()
}
