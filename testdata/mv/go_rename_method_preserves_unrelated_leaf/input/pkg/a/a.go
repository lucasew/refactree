package a

type Driver interface {
	WriteImage()
}

type impl struct{}

func (d *impl) WriteImage() {}

func Use() {
	var d Driver
	d.WriteImage()
}
