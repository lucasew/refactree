package edit

import (
	"fmt"
	"os"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
)

// Options configures a single rft edit run.
type Options struct {
	BaseDir       string
	Input         string // empty → picker over cwd entities
	IncludeHidden bool
	EditorBin     string // --editor flag; empty uses env chain

	// Injectable seams for tests. Nil → production defaults.
	Editor Editor
	Picker Picker
	Getenv func(string) string
	// StreamRefs yields entity reference strings under ref; nil uses WalkSymbols.
	StreamRefs func(baseDir string, ref ingest.Reference, includeHidden bool, emit func(string) error) error
}

// Run opens the definition (or file) for Options.Input, using a picker when needed.
func Run(opts Options) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = "."
	}
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	streamRefs := opts.StreamRefs
	if streamRefs == nil {
		streamRefs = streamEntityRefs
	}

	editor := opts.Editor
	if editor == nil {
		bin, err := ResolveEditorBin(opts.EditorBin, getenv)
		if err != nil {
			return err
		}
		editor = PathLineColumnEditor{Bin: bin}
	}
	picker := opts.Picker
	if picker == nil {
		picker = FZFPicker{}
	}

	input := strings.TrimSpace(opts.Input)
	if input == "" {
		picked, err := picker.Pick(func(emit func(string) error) error {
			return streamRefs(baseDir, ingest.ParseReference("path:./"), opts.IncludeHidden, emit)
		})
		if err != nil {
			return err
		}
		input = picked
	}

	ref := ingest.ParseReference(input)
	ref = ingest.CoerceLocalPathReference(baseDir, ref)
	ref = refpkg.NormalizePathReference(ref)

	if ref.Symbol != "" {
		ref = ingest.CanonicalizeReference(baseDir, ref)
		loc, err := locateDefinition(baseDir, ref)
		if err != nil {
			return err
		}
		return editor.Open(loc)
	}

	// No symbol: file → open 1:1; directory/module → scoped picker.
	abs, isDir, err := isDirRef(baseDir, ref)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path not found: %s", abs)
		}
		return err
	}
	if !isDir {
		return editor.Open(locationFileStart(abs))
	}

	picked, err := picker.Pick(func(emit func(string) error) error {
		return streamRefs(baseDir, ref, opts.IncludeHidden, emit)
	})
	if err != nil {
		return err
	}
	pickedRef := ingest.ParseReference(picked)
	pickedRef = ingest.CanonicalizeReference(baseDir, pickedRef)
	loc, err := locateDefinition(baseDir, pickedRef)
	if err != nil {
		return err
	}
	return editor.Open(loc)
}

// streamEntityRefs walks symbols under ref and emits full reference strings as
// they are discovered (no pre-buffer for fzf).
func streamEntityRefs(baseDir string, ref ingest.Reference, includeHidden bool, emit func(string) error) error {
	scope := ingest.ResolveReferenceScope(baseDir, ref)
	seen := map[string]struct{}{}
	var n int
	var emitErr error
	err := ingest.WalkSymbols(scope.Dir, scope.Reference.String(), ingest.ListOptions{
		IncludeHidden: includeHidden,
		Recursive:     true,
	}, func(sym ingest.SymbolInfo) bool {
		line := sym.Entity.Reference
		if _, ok := seen[line]; ok {
			return true
		}
		seen[line] = struct{}{}
		n++
		if err := emit(line); err != nil {
			emitErr = err
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	if emitErr != nil {
		// Consumer stopped early (fzf closed the pipe); not a listing failure.
		return nil
	}
	if n == 0 {
		return fmt.Errorf("no symbols to edit under %s", ref.String())
	}
	return nil
}
