package pkgb

import "example.com/m/pkgdb"

func SumListed(s *pkgdb.Store) (int, error) {
	boxes, err := s.List()
	if err != nil {
		return 0, err
	}
	sum := 0
	for _, b := range boxes {
		sum += b.Helper + b.Stay
	}
	return sum, nil
}

func UseOne(s *pkgdb.Store) (int, error) {
	b, err := s.Get()
	if err != nil {
		return 0, err
	}
	return b.Helper + b.Stay, nil
}
