package main

type Writer interface {
	WriteData([]byte) (int, error)
}

type Sink struct{}

func (s Sink) WriteData(b []byte) (int, error) { return len(b), nil }

func use(w Writer) {
	_, _ = w.WriteData(nil)
}

func direct(s Sink) {
	_, _ = s.WriteData(nil)
}

func main() {
	use(Sink{})
	direct(Sink{})
}
