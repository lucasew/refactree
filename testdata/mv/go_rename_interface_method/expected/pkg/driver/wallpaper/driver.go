package wallpaper

type Driver interface {
	SetImage(path string) error
}
