package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// make([]T)/make(map) locals and inline index must follow element/value type
// so foreign same-leaf methods on the other collection are not rewritten.
func UseMakeShort() int {
as := make([]*A, 1)
bs := make([]*B, 1)
return as[0].Run() + bs[0].Run()
}

func UseMakeVar() int {
var as = make([]*A, 1)
var bs = make([]*B, 1)
return as[0].Run() + bs[0].Run()
}

func UseMakeCap() int {
as := make([]*A, 1, 2)
bs := make([]*B, 1, 2)
return as[0].Run() + bs[0].Run()
}

func UseMakeMap() int {
ma := make(map[string]*A)
mb := make(map[string]*B)
return ma["k"].Run() + mb["k"].Run()
}

func UseMakeInline() int {
return make([]*A, 1)[0].Run() + make([]*B, 1)[0].Run()
}
