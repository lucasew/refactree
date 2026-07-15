package box

type Box[T any] struct{ V T }

func (b *Box[T]) Helper() int { return 1 }
func (b *Box[T]) Stay() int   { return 2 }

func Use() int {
	b := &Box[int]{}
	return b.Helper() + b.Stay()
}
