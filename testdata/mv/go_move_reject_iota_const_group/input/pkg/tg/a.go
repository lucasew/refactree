package tg

type PoolKind int

const (
	Control  PoolKind = iota
	IO
	CPU
	Internet
)

func Use() PoolKind { return Control }
