package media

type PlaybackStatus string

const (
	StatusPlaying PlaybackStatus = "Playing"
	StatusPaused  PlaybackStatus = "Paused"
)

type Metadata struct {
	Status PlaybackStatus
}

func Current() PlaybackStatus { return StatusPlaying }
