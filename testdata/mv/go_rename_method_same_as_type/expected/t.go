package demo

type Helper struct{ n int }

func (h Helper) Assist() int { return h.n }
func (h Helper) Stay() int   { return 2 }

func Use(h Helper) int {
	return h.Assist() + h.Stay()
}

func Make() Helper {
	return Helper{n: 1}
}
