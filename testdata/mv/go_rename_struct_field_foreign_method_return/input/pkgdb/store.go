package pkgdb

import "example.com/m/pkga"

type Store struct{}

func (s *Store) List() ([]pkga.Box, error) {
	return []pkga.Box{{Helper: 1, Stay: 2}}, nil
}

func (s *Store) Get() (pkga.Box, error) {
	return pkga.Box{Helper: 3, Stay: 4}, nil
}
