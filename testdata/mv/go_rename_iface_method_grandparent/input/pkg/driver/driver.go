package driver

type DriverFactory[T any] interface {
	ID() string
	New() (T, error)
}

func Register[T any](f DriverFactory[T]) {}
