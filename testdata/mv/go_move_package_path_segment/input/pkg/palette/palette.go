package palette

import (
	"example/pkg/api"
	paletteapi "example/pkg/palette/api"
)

func Use() {
	api.Ping()
	_ = paletteapi.Driver{}
}
