package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type SA struct {
	Items []*A
	M     map[string]*A
	Fa    func() []*A
}
type SB struct {
	Items []*B
	M     map[string]*B
	Fa    func() []*B
}

type SAS []SA
type SBS []SB

// Nested named field on slice element: sas[0].Inner.Items
type InnerA struct {
	Items []*A
}
type InnerB struct {
	Items []*B
}
type SA2 struct {
	Inner InnerA
}
type SB2 struct {
	Inner InnerB
}

func UseSlice(sas []SA, sbs []SB) int {
	return sas[0].Items[0].Run() + sbs[0].Items[0].Run()
}

func UseSlicePtr(sas []*SA, sbs []*SB) int {
	return sas[0].Items[0].Run() + sbs[0].Items[0].Run()
}

func UseArray(sas [1]SA, sbs [1]SB) int {
	return sas[0].Items[0].Run() + sbs[0].Items[0].Run()
}

func UseNamedSlice(sas SAS, sbs SBS) int {
	return sas[0].Items[0].Run() + sbs[0].Items[0].Run()
}

func UseSliceMap(sas []SA, sbs []SB) int {
	return sas[0].M["k"].Run() + sbs[0].M["k"].Run()
}

func UseMapOfStruct(mas map[string]SA, mbs map[string]SB) int {
	return mas["k"].Items[0].Run() + mbs["k"].Items[0].Run()
}

func UseMapOfPtr(mas map[string]*SA, mbs map[string]*SB) int {
	return mas["k"].Items[0].Run() + mbs["k"].Items[0].Run()
}

func UseSliceFuncField(sas []SA, sbs []SB) int {
	return sas[0].Fa()[0].Run() + sbs[0].Fa()[0].Run()
}

func UseSliceAssign(sas []SA, sbs []SB) int {
	items := sas[0].Items
	other := sbs[0].Items
	return items[0].Run() + other[0].Run()
}

func UseMake() int {
	sas := make([]SA, 1)
	sbs := make([]SB, 1)
	return sas[0].Items[0].Run() + sbs[0].Items[0].Run()
}

func UseSliceNested(sas []SA2, sbs []SB2) int {
	return sas[0].Inner.Items[0].Run() + sbs[0].Inner.Items[0].Run()
}

func UsePreservesB(sbs []SB, sbs2 []*SB) int {
	return sbs[0].Items[0].Run() + sbs2[0].M["k"].Run()
}
