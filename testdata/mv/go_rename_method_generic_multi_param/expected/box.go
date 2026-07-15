package box

type Map[K comparable, V any] struct{}

func (Map[K, V]) Assist() int { return 1 }
func (Map[K, V]) Stay() int   { return 2 }

func Use() int {
	m := Map[string, int]{}
	return m.Assist() + m.Stay()
}
