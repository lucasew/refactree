package xdg

import "demo/pkg/driver"

func init() {
	driver.Register[Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string { return "xdg" }

func (f *Factory) New() (Driver, error) { return Driver{}, nil }

type Driver struct{}
