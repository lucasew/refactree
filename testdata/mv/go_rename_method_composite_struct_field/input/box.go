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

type SAS []SA
type SBS []SB

func UseComposite() int {
	return []SA{{Items: []*A{{}}}}[0].Items[0].Run() + []SB{{Items: []*B{{}}}}[0].Items[0].Run()
}

func UseCompositePtr() int {
	return []*SA{{Items: []*A{{}}}}[0].Items[0].Run() + []*SB{{Items: []*B{{}}}}[0].Items[0].Run()
}

func UseMapComposite() int {
	return map[string]SA{"k": {Items: []*A{{}}}}["k"].Items[0].Run() + map[string]SB{"k": {Items: []*B{{}}}}["k"].Items[0].Run()
}

func UseMapCompositeVal() int {
	return map[string]SA{"k": {Items: []*A{{}}}}["k"].M["x"].Run() + map[string]SB{"k": {Items: []*B{{}}}}["k"].M["x"].Run()
}

func UseMake() int {
	return make([]SA, 1)[0].Items[0].Run() + make([]SB, 1)[0].Items[0].Run()
}

func UseArray() int {
	return [1]SA{{Items: []*A{{}}}}[0].Items[0].Run() + [1]SB{{Items: []*B{{}}}}[0].Items[0].Run()
}

func UseAppend() int {
	return append([]SA{}, SA{Items: []*A{{}}})[0].Items[0].Run() + append([]SB{}, SB{Items: []*B{{}}})[0].Items[0].Run()
}

func UseCompositeAssign() int {
	sa := []SA{{Items: []*A{{}}}}
	sb := []SB{{Items: []*B{{}}}}
	return sa[0].Items[0].Run() + sb[0].Items[0].Run()
}


func UseNamed() int {
	return SAS{{Items: []*A{{}}}}[0].Items[0].Run() + SBS{{Items: []*B{{}}}}[0].Items[0].Run()
}

func UsePreservesB() int {
	return []SB{{Items: []*B{{}}}}[0].Items[0].Run() + make([]SB, 1)[0].M["k"].Run()
}
