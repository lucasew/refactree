package pkgb

import "example.com/m/pkga"

func Use(b *pkga.Box) int {
	return b.Assist() + b.Stay()
}
