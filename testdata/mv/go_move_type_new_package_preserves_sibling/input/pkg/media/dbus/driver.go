package dbus

import "example.com/m/pkg/media"

func Status() media.PlaybackStatus {
	return media.StatusPlaying
}
