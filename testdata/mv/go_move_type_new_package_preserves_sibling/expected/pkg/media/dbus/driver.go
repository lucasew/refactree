package dbus

import "example.com/m/pkg/media_fuzz"

import "example.com/m/pkg/media"

func Status() media_fuzz.PlaybackStatus {
	return media.StatusPlaying
}
