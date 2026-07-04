package a

type Driver interface {
	Renamed()
}

type impl struct{}

func (d *impl) Renamed() {}

func Use() {
	var d Driver
	d.Renamed()
}
