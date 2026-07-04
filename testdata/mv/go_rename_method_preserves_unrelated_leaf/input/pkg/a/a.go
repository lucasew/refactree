package a

type Driver interface {
	WriteImage()
}

type impl struct{}

func (d *impl) WriteImage() {}

// Ignore errors if service doesn't exist
func Use() {
	var d Driver
	d.WriteImage()
}
