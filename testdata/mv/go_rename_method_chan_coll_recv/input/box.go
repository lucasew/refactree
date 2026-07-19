package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func UseShort(chA <-chan []*A, chB <-chan []*B) int {
	as := <-chA
	bs := <-chB
	return as[0].Run() + bs[0].Run()
}

func UseVar(chA <-chan []*A, chB <-chan []*B) int {
	var as = <-chA
	var bs = <-chB
	return as[0].Run() + bs[0].Run()
}

func UseInline(chA <-chan []*A, chB <-chan []*B) int {
	return (<-chA)[0].Run() + (<-chB)[0].Run()
}

func UseMap(chA <-chan map[string]*A, chB <-chan map[string]*B) int {
	ma := <-chA
	mb := <-chB
	return ma["k"].Run() + mb["k"].Run()
}

func UseNamed(chA <-chan AS, chB <-chan BS) int {
	as := <-chA
	bs := <-chB
	return as[0].Run() + bs[0].Run()
}

type AS []*A
type BS []*B

func UsePreservesB(chB <-chan []*B) int {
	bs := <-chB
	return bs[0].Run() + (<-chB)[0].Run()
}
