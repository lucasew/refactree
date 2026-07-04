package wallpaper

type Driver interface {
	SetStatic(path string) error
}
