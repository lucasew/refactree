package wallpaper

func SetImage(path string) error {
	var d Driver
	return d.SetImage(path)
}
