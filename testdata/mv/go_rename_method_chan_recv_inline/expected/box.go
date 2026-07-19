package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Inline channel receive under foreign same-leaf methods.
// Payload type must match assigned receive short-vars.
func UseInline(ca <-chan *A, cb <-chan *B) int {
	return (<-ca).Execute() + (<-cb).Run()
}

// Nested paren form.
func UseNestedParen(ca <-chan *A, cb <-chan *B) int {
	return ((<-ca)).Execute() + ((<-cb)).Run()
}

// Regression: assigned receive already peels payload type.
func UseVar(ca <-chan *A, cb <-chan *B) int {
	a := <-ca
	b := <-cb
	return a.Execute() + b.Run()
}

// Regression: comma-ok receive.
func UseCommaOk(ca <-chan *A, cb <-chan *B) int {
	a, ok := <-ca
	if !ok {
		return 0
	}
	b, ok2 := <-cb
	if !ok2 {
		return a.Execute()
	}
	return a.Execute() + b.Run()
}
