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

func UseShortVar(sas []SA, sbs []SB) int {
	sa := sas[0]
	sb := sbs[0]
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseShortVarMap(sas []SA, sbs []SB) int {
	sa := sas[0]
	sb := sbs[0]
	return sa.M["k"].Run() + sb.M["k"].Run()
}

func UseVarSpec(sas []SA, sbs []SB) int {
	var sa = sas[0]
	var sb = sbs[0]
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseShortVarPtr(sas []*SA, sbs []*SB) int {
	sa := sas[0]
	sb := sbs[0]
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseShortVarMapOfStruct(mas map[string]SA, mbs map[string]SB) int {
	sa := mas["k"]
	sb := mbs["k"]
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseShortVarNested(sas []SA, sbs []SB) int {
	sa := sas[0]
	items := sa.Items
	other := sbs[0].Items
	return items[0].Run() + other[0].Run()
}

func UseShortFromMake() int {
	sas := make([]SA, 1)
	sbs := make([]SB, 1)
	sa := sas[0]
	sb := sbs[0]
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseMapOK(mas map[string]SA, mbs map[string]SB) int {
	sa, ok := mas["k"]
	if !ok {
		return 0
	}
	sb, ok2 := mbs["k"]
	if !ok2 {
		return 0
	}
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseElemShortVar(as []*A, bs []*B) int {
	a := as[0]
	b := bs[0]
	return a.Run() + b.Run()
}

func UsePreservesB(sbs []SB) int {
	sb := sbs[0]
	return sb.Items[0].Run() + sb.M["k"].Run()
}
