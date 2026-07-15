package box

type Box struct{}

func (b Box) Helper() int { return 1 }
func (b Box) Stay() int   { return 2 }

func Use(v any) int {
	return (v.(Box)).Helper() + (v.(Box)).Stay()
}
