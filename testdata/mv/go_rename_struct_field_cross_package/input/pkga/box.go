package pkga

type Box struct {
	Helper int
	Stay   int
}

type Provider interface {
	Resolve() ([]Box, error)
}
