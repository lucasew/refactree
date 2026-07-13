package main

import (
	cmdwall "example/cmd/app/driver/wallpaper_fuzz"
	"example/pkg/driver/wallpaper"
)

func main() {
	cmdwall.Run()
	wallpaper.Set()
}
