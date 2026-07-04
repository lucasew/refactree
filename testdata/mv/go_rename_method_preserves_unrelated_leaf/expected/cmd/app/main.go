package main

import (
	"example/pkg/a"
	"example/pkg/b"
)

func main() {
	var d a.Driver
	d.Renamed()
	b.Unrelated{}.WriteImage()
	b.WriteImage()
	_ = "pkg.a.WriteImage"
}
