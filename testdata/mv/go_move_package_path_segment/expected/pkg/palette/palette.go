package palette

import (
	"example/pkg/api_fuzz"
	paletteapi "example/pkg/palette/api"
)

func Use() {
	api.Ping()
	_ = paletteapi.Driver{}
}
