package main

type A struct{}
type B struct{}

func (a A) Run() int {
	return 1
}

func (b *B) Run() int {
	return 2
}
