package a

type Driver interface {
	Renamed()
}

type impl struct{}

func (d *impl) Renamed() {}

// Ignore errors if service doesn't exist
func Use() {
	var d Driver
	d.Renamed()
}
