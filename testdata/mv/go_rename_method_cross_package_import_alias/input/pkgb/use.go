package pkgb

import a "example.com/m/pkga"

func Use() int {
	b := a.Box{}
	return b.Helper() + b.Stay()
}
