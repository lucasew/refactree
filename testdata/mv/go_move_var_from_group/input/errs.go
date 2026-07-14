package main

import "errors"

var (
	ErrNoVersions     = errors.New("no versions found")
	ErrNoGoVersions   = ErrNoVersions
	ErrNoNodeVersions = ErrNoVersions
)
