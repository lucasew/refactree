package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type AM map[string]*A
type BM map[string]*B

type SA struct {
	Items []*A
	Named AS
	M     map[string]*A
	NamedM AM
}
type SB struct {
	Items []*B
	Named BS
	M     map[string]*B
	NamedM BM
}

// Embed promotes collection fields under foreign same-leaf.
type WA struct {
	SA
}
type WB struct {
	SB
}

func UseDirect(sa SA, sb SB) int {
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseNamed(sa SA, sb SB) int {
	return sa.Named[0].Run() + sb.Named[0].Run()
}

func UseMap(sa SA, sb SB) int {
	return sa.M["k"].Run() + sb.M["k"].Run()
}

func UseNamedMap(sa SA, sb SB) int {
	return sa.NamedM["k"].Run() + sb.NamedM["k"].Run()
}

func UsePtr(sa *SA, sb *SB) int {
	return sa.Items[0].Run() + sb.Items[0].Run()
}

func UseAssign(sa SA, sb SB) int {
	items := sa.Items
	other := sb.Items
	return items[0].Run() + other[0].Run()
}

func UseVar(sa SA, sb SB) int {
	var items = sa.Items
	var other = sb.Items
	return items[0].Run() + other[0].Run()
}

func UseEmbed(wa WA, wb WB) int {
	return wa.Items[0].Run() + wb.Items[0].Run()
}

func UseEmbedMap(wa WA, wb WB) int {
	return wa.M["k"].Run() + wb.M["k"].Run()
}

func UsePreservesB(sb SB, wb WB) int {
	return sb.Items[0].Run() + wb.Named[0].Run() + sb.M["k"].Run()
}
