package main

type Writer interface {
	Write([]byte) (int, error)
}

type Sink struct{}

func (s Sink) Write(b []byte) (int, error) { return len(b), nil }

func use(w Writer) {
	_, _ = w.Write(nil)
}

func direct(s Sink) {
	_, _ = s.Write(nil)
}

func main() {
	use(Sink{})
	direct(Sink{})
}
