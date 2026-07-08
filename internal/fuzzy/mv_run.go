package fuzzy

import (
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// MvRunOptions configures move fuzzing.
type MvRunOptions struct {
	StrictRefs bool
	Grains     []string
}

// movePlan is the logged plan: placement plus source/destination references.
type movePlan struct {
	Placement   Placement
	Source      string
	Destination string
}

// PlanInput is the minimizable decision surface shared by catalog RNG iterations
// and Go native fuzz (testing.F). Indices are taken mod available options.
type PlanInput struct {
	GrainIndex     uint8
	SourceIndex    uint32
	PlacementIndex uint8
	PeerIndex      uint32
	Entropy        uint32
}

// PlanInputFromRand draws a PlanInput from a seeded RNG (catalog ModeMv/ModeRun).
func PlanInputFromRand(rng *rand.Rand) PlanInput {
	return PlanInput{
		GrainIndex:     uint8(rng.Intn(256)),
		SourceIndex:    uint32(rng.Uint32()),
		PlacementIndex: uint8(rng.Intn(256)),
		PeerIndex:      uint32(rng.Uint32()),
		Entropy:        uint32(rng.Uint32()),
	}
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
		"no movable",
		"no placement",
		"no source nodes",
	}
	for _, s := range unsupported {
		if strings.Contains(msg, s) {
			return "unsupported"
		}
	}
	return "bug"
}

func pickMvPlan(rng *rand.Rand, p Project, root string, result *ingest.Result) (movePlan, error) {
	return pickMvPlanWith(PlanInputFromRand(rng), p, root, result)
}

func pickMvPlanWith(in PlanInput, p Project, root string, result *ingest.Result) (movePlan, error) {
	_ = root
	model, err := moveModelForLanguage(p.Language)
	if err != nil {
		return movePlan{}, err
	}

	grains, err := resolveProjectGrains(p, model)
	if err != nil {
		return movePlan{}, err
	}
	grain := grains[int(in.GrainIndex)%len(grains)]

	nodes := model.ListNodes(result, grain, p.Language)
	if len(nodes) == 0 {
		return movePlan{}, fmt.Errorf("no source nodes for grain %s language %s", grain, p.Language)
	}
	source := nodes[int(in.SourceIndex)%len(nodes)]

	placements := placementMenu(grain, model, result, p.Language, source)
	if len(p.Mv.Placements) > 0 {
		placements = filterPlacements(placements, p.Mv.Placements)
	}
	if len(placements) == 0 {
		return movePlan{}, fmt.Errorf("no placements for grain %s", grain)
	}
	placement := placements[int(in.PlacementIndex)%len(placements)]

	return materializeMovePlan(in, model, result, p.Language, source, placement)
}

func resolveProjectGrains(p Project, model languageMoveModel) ([]Grain, error) {
	allowed := model.Grains()
	if len(p.Mv.Grains) == 0 {
		return nil, fmt.Errorf("no mv grains configured")
	}
	var out []Grain
	for _, name := range p.Mv.Grains {
		g := Grain(name)
		ok := false
		for _, a := range allowed {
			if a == g {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("grain %q not valid for language %s", name, p.Language)
		}
		out = append(out, g)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no mv grains configured")
	}
	return out, nil
}

func filterPlacements(menu []Placement, allow []string) []Placement {
	set := map[string]bool{}
	for _, a := range allow {
		set[a] = true
	}
	var out []Placement
	for _, p := range menu {
		if set[string(p)] {
			out = append(out, p)
		}
	}
	return out
}

// placementMenu lists valid placements for this source node.
func placementMenu(grain Grain, model languageMoveModel, result *ingest.Result, projectLanguage string, source MoveNode) []Placement {
	switch grain {
	case GrainPackage:
		return []Placement{PlacementPackage}
	case GrainModule:
		return []Placement{PlacementModule, PlacementNewModule}
	case GrainDeclaration:
		var menu []Placement
		menu = append(menu, PlacementRename)
		// layout: only when the language can place a declaration in another file
		// inside the same module (Go/Java packages with multiple files, or new file).
		modKey := model.ModuleKey(source.Path)
		sameFiles := filesInModule(result, model, projectLanguage, modKey)
		// Offer layout if same-module has another file OR we can create a new layout file
		// (always true for directory modules; for file-modules layout is impossible).
		if _, isFileModule := model.(ecmaMoveModel); isFileModule {
			// module == file: layout cannot differ from module boundary
		} else if _, isPy := model.(pythonMoveModel); isPy {
			// Python declaration lives in a file-module; moving to another file is module placement.
		} else {
			// go / jvm: layout within package
			menu = append(menu, PlacementLayout)
		}
		if len(modulesOtherThan(result, model, projectLanguage, modKey)) > 0 || len(sameFiles) > 1 {
			menu = append(menu, PlacementModule)
		} else if len(listLanguageFiles(result, projectLanguage)) > 1 {
			menu = append(menu, PlacementModule)
		}
		menu = append(menu, PlacementNewModule)
		return menu
	default:
		return nil
	}
}

func materializeMovePlan(in PlanInput, model languageMoveModel, result *ingest.Result, projectLanguage string, source MoveNode, placement Placement) (movePlan, error) {
	switch source.Grain {
	case GrainPackage:
		return materializePackagePlan(in, source, placement)
	case GrainModule:
		return materializeModuleFilePlan(in, result, model, projectLanguage, source, placement)
	case GrainDeclaration:
		return materializeDeclarationPlan(in, model, result, projectLanguage, source, placement)
	default:
		return movePlan{}, fmt.Errorf("unsupported grain %s", source.Grain)
	}
}

func materializePackagePlan(in PlanInput, source MoveNode, placement Placement) (movePlan, error) {
	if placement != PlacementPackage {
		return movePlan{}, fmt.Errorf("placement %s not valid for package grain", placement)
	}
	srcDir := strings.TrimSuffix(source.Path, "/")
	dstDir := fmt.Sprintf("%s_fuzz_%x", srcDir, in.Entropy&0xffff)
	return movePlan{
		Placement:   PlacementPackage,
		Source:      pathReference(srcDir),
		Destination: pathReference(dstDir),
	}, nil
}

func materializeModuleFilePlan(in PlanInput, result *ingest.Result, model languageMoveModel, projectLanguage string, source MoveNode, placement Placement) (movePlan, error) {
	srcPath := source.Path
	var dstPath string
	switch placement {
	case PlacementModule:
		// Peer file path in another module (another file).
		files := listLanguageFiles(result, projectLanguage)
		var peers []string
		for _, f := range files {
			if !model.SameModule(f, srcPath) {
				peers = append(peers, f)
			}
		}
		if len(peers) == 0 {
			// Fall back to new module path beside source.
			dstPath = newSiblingPath(srcPath, in.Entropy)
			placement = PlacementNewModule
		} else {
			// Relocate source file to a new path next to a peer (same dir as peer, new name).
			peer := peers[int(in.PeerIndex)%len(peers)]
			ext := filepath.Ext(srcPath)
			base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
			dstPath = "./" + path.Join(path.Dir(strings.TrimPrefix(peer, "./")), fmt.Sprintf("%s_fuzz_%x%s", base, in.Entropy&0xffff, ext))
		}
	case PlacementNewModule:
		dstPath = newSiblingPath(srcPath, in.Entropy)
	default:
		return movePlan{}, fmt.Errorf("placement %s not valid for module grain", placement)
	}
	return movePlan{
		Placement:   placement,
		Source:      pathReference(srcPath),
		Destination: pathReference(dstPath),
	}, nil
}

func materializeDeclarationPlan(in PlanInput, model languageMoveModel, result *ingest.Result, projectLanguage string, source MoveNode, placement Placement) (movePlan, error) {
	symbolNames := map[string]bool{}
	for _, n := range listDeclarationNodes(result, projectLanguage) {
		symbolNames[n.Symbol] = true
	}

	srcPath := source.Path
	srcSymbol := source.Symbol
	modKey := model.ModuleKey(srcPath)

	switch placement {
	case PlacementRename:
		leaf := srcSymbol
		if i := strings.LastIndex(leaf, "."); i >= 0 {
			leaf = leaf[i+1:]
		}
		leaf = strings.TrimPrefix(leaf, "*")
		name := uniqueSymbolFrom(in.Entropy, symbolNames, leaf)
		if i := strings.LastIndex(srcSymbol, "."); i >= 0 {
			name = srcSymbol[:i+1] + name
		}
		return movePlan{
			Placement:   PlacementRename,
			Source:      source.Reference,
			Destination: ingest.SymbolRef(srcPath, name),
		}, nil

	case PlacementLayout:
		// Same module, different layout file, keep name.
		sameFiles := filesInModule(result, model, projectLanguage, modKey)
		dstPath := peerFileInModule(sameFiles, srcPath, in.PeerIndex)
		if dstPath == "" {
			dstPath = newLayoutFileInModule(srcPath, in.Entropy)
		}
		return movePlan{
			Placement:   PlacementLayout,
			Source:      source.Reference,
			Destination: ingest.SymbolRef(dstPath, srcSymbol),
		}, nil

	case PlacementModule:
		// Different existing module, keep name.
		others := modulesOtherThan(result, model, projectLanguage, modKey)
		var dstPath string
		if len(others) == 0 {
			// No other module: try any other file (different path).
			for _, f := range listLanguageFiles(result, projectLanguage) {
				if f != srcPath {
					dstPath = f
					break
				}
			}
			if dstPath == "" {
				return movePlan{}, fmt.Errorf("no existing peer module for placement module")
			}
		} else {
			targetMod := others[int(in.PeerIndex)%len(others)]
			dstPath = fileInModuleKey(result, model, projectLanguage, targetMod, in.PeerIndex)
			if dstPath == "" {
				return movePlan{}, fmt.Errorf("no file in peer module %s", targetMod)
			}
		}
		return movePlan{
			Placement:   PlacementModule,
			Source:      source.Reference,
			Destination: ingest.SymbolRef(dstPath, srcSymbol),
		}, nil

	case PlacementNewModule:
		dstPath := newModulePathForDeclaration(model, srcPath, in.Entropy)
		return movePlan{
			Placement:   PlacementNewModule,
			Source:      source.Reference,
			Destination: ingest.SymbolRef(dstPath, srcSymbol),
		}, nil

	default:
		return movePlan{}, fmt.Errorf("placement %s not valid for declaration grain", placement)
	}
}

func pathReference(p string) string {
	if !strings.HasPrefix(p, "./") {
		p = "./" + strings.TrimPrefix(p, "/")
	}
	return "path:" + p
}

func newSiblingPath(srcPath string, entropy uint32) string {
	ext := filepath.Ext(srcPath)
	base := strings.TrimSuffix(filepath.Base(srcPath), ext)
	dir := path.Dir(strings.TrimPrefix(srcPath, "./"))
	name := fmt.Sprintf("%s_fuzz_%x%s", base, entropy&0xffff, ext)
	if dir == "." {
		return "./" + name
	}
	return "./" + path.Join(dir, name)
}

func newLayoutFileInModule(srcPath string, entropy uint32) string {
	return newSiblingPath(srcPath, entropy)
}

func newModulePathForDeclaration(model languageMoveModel, srcPath string, entropy uint32) string {
	// Go/Java: new directory (new package) + same basename.
	// JS/Python file-module model: new file path (new module).
	switch model.(type) {
	case goMoveModel, jvmMoveModel:
		rel := strings.TrimPrefix(srcPath, "./")
		dir := path.Dir(rel)
		base := filepath.Base(rel)
		parent := path.Dir(dir)
		pkg := path.Base(dir)
		newPkg := fmt.Sprintf("%s_fuzz_%x", pkg, entropy&0xffff)
		if parent == "." {
			return "./" + path.Join(newPkg, base)
		}
		return "./" + path.Join(parent, newPkg, base)
	default:
		return newSiblingPath(srcPath, entropy)
	}
}

func uniqueSymbolFrom(entropy uint32, existing map[string]bool, styleHint string) string {
	for i := uint32(0); i < 1000; i++ {
		n := int((entropy + i) & ((1 << 20) - 1))
		name := formatFuzzName(styleHint, n)
		if !existing[name] {
			return name
		}
	}
	return formatFuzzName(styleHint, int(entropy))
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
		return "Fuzz" + hex
	}
	if strings.ToLower(styleHint[:1]) == styleHint[:1] && !strings.Contains(styleHint, "_") {
		return "fuzz" + hex
	}
	if strings.ToUpper(styleHint) == styleHint {
		return "FUZZ" + strings.ToUpper(hex)
	}
	return "fuzz_" + hex
}

// ApplyMvPlan runs Rename+ApplyEdits and returns edits.
func ApplyMvPlan(root string, plan movePlan) ([]ingest.Edit, error) {
	edits, err := ingest.Rename(root, plan.Source, plan.Destination)
	if err != nil {
		return nil, err
	}
	if err := ingest.ApplyEdits(root, edits); err != nil {
		return edits, err
	}
	return edits, nil
}

func postMvInvariants(root string, plan movePlan, strict bool) []InvariantFailure {
	result, err := ingest.Ingest(root)
	if err != nil {
		return []InvariantFailure{{Check: "post_ingest", Message: err.Error()}}
	}
	fails := CheckInvariants(root, result, InvariantOptions{StrictRefs: strict})
	entityRefs := map[string]bool{}
	for _, e := range result.Entities {
		entityRefs[e.Reference] = true
	}
	dst := ingest.ParseReference(plan.Destination)
	// Package/module file relocate (no symbol): skip declaration presence checks.
	if plan.Placement == PlacementPackage || dst.Symbol == "" {
		return fails
	}
	if entityRefs[plan.Source] {
		fails = append(fails, InvariantFailure{Check: "source_removed", Message: plan.Source + " still present"})
	}
	if dst.Symbol != "" && !entityRefs[plan.Destination] {
		found := false
		for ref := range entityRefs {
			r := ingest.ParseReference(ref)
			if r.Symbol == dst.Symbol && strings.TrimPrefix(r.Path, "./") == strings.TrimPrefix(dst.Path, "./") {
				found = true
				break
			}
		}
		if !found {
			fails = append(fails, InvariantFailure{Check: "dest_present", Message: plan.Destination + " missing"})
		}
	}
	return fails
}
