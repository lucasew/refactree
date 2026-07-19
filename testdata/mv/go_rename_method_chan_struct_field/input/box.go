package box

type A struct{}

func (a *A) Run() int { return 1 }

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

func UseChanInline(chA <-chan SA, chB <-chan SB) int {
	return (<-chA).Items[0].Run() + (<-chB).Items[0].Run()
}

func UseChanMap(chA <-chan SA, chB <-chan SB) int {
	return (<-chA).M["k"].Run() + (<-chB).M["k"].Run()
}

func UseChanShort(chA <-chan SA, chB <-chan SB) int {
	sa := <-chA
	sb := <-chB
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseChanPtr(chA <-chan *SA, chB <-chan *SB) int {
	return (<-chA).Items[0].Run() + (<-chB).Items[0].Run()
}

func UseChanAssignField(chA <-chan SA, chB <-chan SB) int {
	items := (<-chA).Items
	other := (<-chB).Items
	return items[0].Run() + other[0].Run()
}

func UsePreservesB(chB <-chan SB) int {
	return (<-chB).Items[0].Run() + (<-chB).M["k"].Run()
}
