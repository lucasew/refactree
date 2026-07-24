package driver

type DriverFactory[T any] interface {
	ID() string
	Make() (T, error)
}

func Register[T any](f DriverFactory[T]) {}
