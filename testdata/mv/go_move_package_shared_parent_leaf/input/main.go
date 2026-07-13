package main

import (
	cmdwall "example/cmd/app/driver/wallpaper"
	"example/pkg/driver/wallpaper"
)

func main() {
	cmdwall.Run()
	wallpaper.Set()
}
