package main

import "errors"

var (
	ErrNoVersions     = errors.New("no versions found")
	ErrNoNodeVersions = ErrNoVersions
)
