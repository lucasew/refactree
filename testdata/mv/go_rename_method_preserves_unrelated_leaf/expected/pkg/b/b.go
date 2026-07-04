package b

type Unrelated struct{}

func (Unrelated) WriteImage() {}

func WriteImage() {}

const msg = "a.WriteImage"

func Other() {
	Unrelated{}.WriteImage()
	WriteImage()
}
