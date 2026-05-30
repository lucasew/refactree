package ingest

// MoveDriver defines language-specific cross-file move planning.
type MoveDriver interface {
	Language() string
	PlanCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Entity) ([]Edit, error)
}

type goMoveDriver struct{}

func (goMoveDriver) Language() string { return "go" }

func (goMoveDriver) PlanCrossFileMove(dir string, result *Result, src, dst Reference, sourceEntity Entity) ([]Edit, error) {
	return planGoCrossFileMove(dir, result, src, dst, sourceEntity)
}

func moveDriverForLanguage(lang string) (MoveDriver, bool) {
	d, ok := moveDrivers[lang]
	return d, ok
}

var moveDrivers = map[string]MoveDriver{
	"go": goMoveDriver{},
}
