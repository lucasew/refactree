package media

import "example.com/m/pkg/media_fuzz"

const (
	StatusPlaying media_fuzz.PlaybackStatus = "Playing"
	StatusPaused  media_fuzz.PlaybackStatus = "Paused"
)

type Metadata struct {
	Status media_fuzz.PlaybackStatus
}

func Current() media_fuzz.PlaybackStatus { return StatusPlaying }
