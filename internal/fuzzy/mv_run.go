package fuzzy

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// MvRunOptions configures move fuzzing.
type MvRunOptions struct {
	StrictRefs bool
	Ops        []string
}

type mvPlan struct {
	Op  string
	Src string
	Dst string
}

func classifyMvError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	unsupported := []string{
		"not supported",
		"unsupported",
		"ambiguous",
		"no entity found",
		"must both include symbols",
		"package move requires",
	}
	for _, s := range unsupported {
		if strings.Contains(msg, s) {
			return "unsupported"
		}
	}
	return "bug"
}

func pickMvPlan(rng *rand.Rand, p Project, root string, result *ingest.Result, ops []string) (mvPlan, error) {
	if len(ops) == 0 {
		ops = p.Mv.Ops
	}
	if len(ops) == 0 {
		return mvPlan{}, fmt.Errorf("no mv ops configured")
	}
	op := ops[rng.Intn(len(ops))]

	var entities []ingest.Entity
	symbolNames := map[string]bool{}
	filesByLang := map[string][]string{}
	for _, f := range result.Files {
		if f.Language == p.Language {
			filesByLang[f.Language] = append(filesByLang[f.Language], f.Path)
		}
	}
	for _, e := range result.Entities {
		ref := ingest.ParseReference(e.Reference)
		if ref.Provider != "path" || ref.Symbol == "" {
			continue
		}
		rel := strings.TrimPrefix(ref.Path, "./")
		lang, ok := ingest.LanguageForFile(rel)
		if !ok || lang != p.Language {
			continue
		}
		if strings.HasPrefix(ref.Symbol, "fuzz_") {
			continue
		}
		entities = append(entities, e)
		symbolNames[ref.Symbol] = true
	}
	if len(entities) == 0 {
		return mvPlan{}, fmt.Errorf("no movable entities for language %s", p.Language)
	}

	ent := entities[rng.Intn(len(entities))]
	srcRef := ingest.ParseReference(ent.Reference)
	srcPath := srcRef.Path
	if !strings.HasPrefix(srcPath, "./") {
		srcPath = "./" + srcPath
	}

	switch op {
	case "rename":
		leaf := srcRef.Symbol
		if i := strings.LastIndex(leaf, "."); i >= 0 {
			leaf = leaf[i+1:]
		}
		leaf = strings.TrimPrefix(leaf, "*")
		name := uniqueSymbol(rng, symbolNames, leaf)
		// Preserve qualifiers for nested symbols (e.g. Java Type.method).
		if i := strings.LastIndex(srcRef.Symbol, "."); i >= 0 {
			name = srcRef.Symbol[:i+1] + name
		}
		symbolNames[name] = true
		return mvPlan{
			Op:  op,
			Src: ent.Reference,
			Dst: ingest.SymbolRef(srcPath, name),
		}, nil
	case "cross_file":
		files := filesByLang[p.Language]
		if len(files) == 0 {
			return mvPlan{}, fmt.Errorf("no destination files")
		}
		var dstPath string
		for tries := 0; tries < 16; tries++ {
			cand := files[rng.Intn(len(files))]
			if !strings.HasPrefix(cand, "./") {
				cand = "./" + cand
			}
			if cand != srcPath {
				dstPath = cand
				break
			}
		}
		if dstPath == "" {
			// new sibling file
			ext := filepath.Ext(srcPath)
			base := strings.TrimSuffix(filepath.Base(srcPath), ext)
			dstPath = "./" + filepath.ToSlash(filepath.Join(filepath.Dir(strings.TrimPrefix(srcPath, "./")), fmt.Sprintf("%s_fuzz_%d%s", base, rng.Intn(1<<16), ext)))
		}
		return mvPlan{
			Op:  op,
			Src: ent.Reference,
			Dst: ingest.SymbolRef(dstPath, srcRef.Symbol),
		}, nil
	case "package":
		srcDir := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(srcPath, "./")))
		if srcDir == "./." || srcDir == "./" {
			return mvPlan{}, fmt.Errorf("entity not in a package directory")
		}
		dstDir := srcDir + "_fuzz"
		return mvPlan{
			Op:  op,
			Src: "path:" + strings.TrimSuffix(srcDir, "/"),
			Dst: "path:" + dstDir,
		}, nil
	default:
		return mvPlan{}, fmt.Errorf("unknown op %q", op)
	}
}

func uniqueSymbol(rng *rand.Rand, existing map[string]bool, styleHint string) string {
	for i := 0; i < 1000; i++ {
		n := rng.Intn(1 << 20)
		name := formatFuzzName(styleHint, n)
		if !existing[name] {
			return name
		}
	}
	return formatFuzzName(styleHint, int(rng.Int63()))
}

func formatFuzzName(styleHint string, n int) string {
	hex := fmt.Sprintf("%x", n)
	if styleHint == "" {
		return "fuzz_" + hex
	}
	if strings.ToUpper(styleHint) == styleHint && strings.Contains(styleHint, "_") {
		return "FUZZ_" + strings.ToUpper(hex)
	}
	if strings.ToUpper(styleHint[:1]) == styleHint[:1] && strings.ToLower(styleHint[1:]) != styleHint[1:] {
		// PascalCase
		return "Fuzz" + hex
	}
	if strings.ToLower(styleHint[:1]) == styleHint[:1] && !strings.Contains(styleHint, "_") {
		// camelCase — ErrorProne accepts lowerCamelCase for non-immutable statics.
		return "fuzz" + hex
	}
	if strings.ToUpper(styleHint) == styleHint {
		return "FUZZ" + strings.ToUpper(hex)
	}
	return "fuzz_" + hex
}

// ApplyMvPlan runs Rename+ApplyEdits and returns edits.
func ApplyMvPlan(root string, plan mvPlan) ([]ingest.Edit, error) {
	edits, err := ingest.Rename(root, plan.Src, plan.Dst)
	if err != nil {
		return nil, err
	}
	if err := ingest.ApplyEdits(root, edits); err != nil {
		return edits, err
	}
	return edits, nil
}

func postMvInvariants(root string, plan mvPlan, strict bool) []InvariantFailure {
	result, err := ingest.Ingest(root)
	if err != nil {
		return []InvariantFailure{{Check: "post_ingest", Message: err.Error()}}
	}
	fails := CheckInvariants(root, result, InvariantOptions{StrictRefs: strict})
	entityRefs := map[string]bool{}
	for _, e := range result.Entities {
		entityRefs[e.Reference] = true
	}
	dst := ingest.ParseReference(plan.Dst)
	if plan.Op == "package" {
		return fails
	}
	if entityRefs[plan.Src] {
		fails = append(fails, InvariantFailure{Check: "source_removed", Message: plan.Src + " still present"})
	}
	if dst.Symbol != "" && !entityRefs[plan.Dst] {
		found := false
		for ref := range entityRefs {
			r := ingest.ParseReference(ref)
			if r.Symbol == dst.Symbol && strings.TrimPrefix(r.Path, "./") == strings.TrimPrefix(dst.Path, "./") {
				found = true
				break
			}
		}
		if !found {
			fails = append(fails, InvariantFailure{Check: "dest_present", Message: plan.Dst + " missing"})
		}
	}
	return fails
}
