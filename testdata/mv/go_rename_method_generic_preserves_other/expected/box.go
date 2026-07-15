package box

type Box[T any] struct{ V T }

func (Box[T]) Assist() int { return 1 }
func (Box[T]) Stay() int   { return 2 }

type Other[T any] struct{ V T }

func (Other[T]) Helper() int { return 9 }

func Use() int {
	b := Box[int]{}
	o := Other[int]{}
	return b.Assist() + o.Helper() + b.Stay()
}
