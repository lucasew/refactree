package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type SA struct {
	Items []*A
	M     map[string]*A
}
type SB struct {
	Items []*B
	M     map[string]*B
}

func (s SA) Self() SA  { return s }
func (s SB) Self() SB  { return s }
func (s *SA) SelfP() *SA { return s }
func (s *SB) SelfP() *SB { return s }

func UseInline(sa SA, sb SB) int {
	return sa.Self().Items[0].Execute() + sb.Self().Items[0].Run()
}

func UseMap(sa SA, sb SB) int {
	return sa.Self().M["k"].Execute() + sb.Self().M["k"].Run()
}

func UsePtr(sa *SA, sb *SB) int {
	return sa.SelfP().Items[0].Execute() + sb.SelfP().Items[0].Run()
}

func UseAssignField(sa SA, sb SB) int {
	items := sa.Self().Items
	other := sb.Self().Items
	return items[0].Execute() + other[0].Run()
}

func UsePreservesB(sb SB) int {
	return sb.Self().Items[0].Run() + sb.Self().M["k"].Run()
}
