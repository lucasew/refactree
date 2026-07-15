package box

type Box struct{}

func (b Box) Assist() int { return 1 }
func (b Box) Stay() int   { return 2 }

func Use(v any) int {
	return (v.(Box)).Assist() + (v.(Box)).Stay()
}
