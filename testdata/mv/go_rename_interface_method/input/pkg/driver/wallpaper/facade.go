package wallpaper

func SetStatic(path string) error {
	var d Driver
	return d.SetStatic(path)
}
