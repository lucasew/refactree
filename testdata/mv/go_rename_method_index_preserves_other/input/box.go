package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Slice/map index receivers must follow element/value type so foreign
// same-leaf methods on the other collection are not rewritten, while
// ours are.
func Use(as []*A, bs []*B, ma map[string]*A, mb map[string]*B) int {
return as[0].Run() + bs[0].Run() + ma["k"].Run() + mb["k"].Run()
}

func UseLocal() int {
var as []*A
var bs []*B
var ma map[string]*A
var mb map[string]*B
return as[0].Run() + bs[0].Run() + ma["k"].Run() + mb["k"].Run()
}
