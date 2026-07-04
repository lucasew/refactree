package checks

import "example/pkg/cas"

func Run(x int) {
	switch x {
	case 1:
		cas.Store()
	case 2:
		println("github:lucasew/nixcfg")
	}
}
