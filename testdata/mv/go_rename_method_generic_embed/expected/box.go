package box

type Inner[T any] struct{}

func (Inner[T]) Assist() int { return 1 }
func (Inner[T]) Stay() int   { return 2 }

type Box[T any] struct{ Inner[T] }

func Use() int {
	b := Box[int]{}
	return b.Assist() + b.Stay()
}
