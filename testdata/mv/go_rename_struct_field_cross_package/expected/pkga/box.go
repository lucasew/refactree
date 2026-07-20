package pkga

type Box struct {
	Assist int
	Stay   int
}

type Provider interface {
	Resolve() ([]Box, error)
}
