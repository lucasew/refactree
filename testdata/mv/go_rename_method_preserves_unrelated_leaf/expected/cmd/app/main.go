package main

import (
	"example/pkg/a"
	"example/pkg/b"
)

func main() {
	var d a.Driver
	d.WriteImage()
	b.Unrelated{}.WriteImage()
	b.WriteImage()
	_ = "pkg.a.WriteImage"
}
