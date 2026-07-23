package pattern

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

func init() {
	// mv use sites share the Rule site engine when this package is linked.
	ingest.RegisterUseSiteRenamer(UseSiteRenames)
}

// UseSiteRenames rewrites identifier leaves for symbols in sourceSet using
// RefLeafRule (@target → newLeaf) per file — the same site backbone as rewrite.
//
// Files are taken from non-alias Uses that target sourceSet. ViaImportAlias
// spans are excluded so local import aliases keep their binding names.
func UseSiteRenames(root string, result *ingest.Result, sourceSet map[string]bool, newLeaf string) []ingest.Edit {
	if result == nil || len(sourceSet) == 0 || newLeaf == "" {
		return nil
	}

	var targets []string
	for t := range sourceSet {
		if t != "" {
			targets = append(targets, t)
		}
	}
	slices.Sort(targets)

	// file → spans to skip (import-alias bindings)
	skip := map[string]map[ingest.Span]struct{}{}
	files := map[string]struct{}{}
	for _, u := range result.Uses {
		if !sourceSet[u.Target] || u.StartByte >= u.EndByte {
			continue
		}
		ref := ingest.ParseReference(u.Reference)
		file := strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")
		if file == "" {
			continue
		}
		sp := ingest.Span{StartByte: u.StartByte, EndByte: u.EndByte}
		if u.ViaImportAlias {
			if skip[file] == nil {
				skip[file] = map[ingest.Span]struct{}{}
			}
			skip[file][sp] = struct{}{}
			continue
		}
		files[file] = struct{}{}
	}
	if len(files) == 0 {
		return nil
	}

	langByFile := map[string]string{}
	for _, f := range result.Files {
		p := strings.TrimPrefix(filepath.ToSlash(f.Path), "./")
		langByFile[p] = f.Language
	}

	var fileList []string
	for f := range files {
		fileList = append(fileList, f)
	}
	slices.Sort(fileList)

	var edits []ingest.Edit
	for _, file := range fileList {
		abs := file
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, filepath.FromSlash(file))
		}
		source, err := os.ReadFile(abs)
		if err != nil {
			// Fall back to graph spans for this file if unreadable.
			edits = append(edits, graphUseEditsForFile(result, sourceSet, newLeaf, file)...)
			continue
		}
		lang := langByFile[file]
		if lang == "" {
			edits = append(edits, graphUseEditsForFile(result, sourceSet, newLeaf, file)...)
			continue
		}
		pf, err := ingest.ParseSourceFile(abs, lang)
		if err != nil {
			edits = append(edits, graphUseEditsForFile(result, sourceSet, newLeaf, file)...)
			continue
		}

		skipSpans := skip[file]
		for _, target := range targets {
			rule, err := RefLeafRule(target, newLeaf)
			if err != nil {
				continue
			}
			_, fileEdits, err := rule.ExpandFile(root, file, source, pf.Root, result)
			if err != nil {
				continue
			}
			for _, e := range fileEdits {
				if _, bad := skipSpans[e.Span]; bad {
					continue
				}
				edits = append(edits, e)
			}
		}
		pf.Close()
	}
	return edits
}

// graphUseEditsForFile is a per-file fallback matching useSiteRenamesFromGraph.
func graphUseEditsForFile(result *ingest.Result, sourceSet map[string]bool, newLeaf, file string) []ingest.Edit {
	var edits []ingest.Edit
	for _, rel := range result.Uses {
		if !sourceSet[rel.Target] || rel.ViaImportAlias {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		f := strings.TrimPrefix(filepath.ToSlash(ref.Path), "./")
		if f != file {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:    file,
			Span:    ingest.Span{StartByte: rel.StartByte, EndByte: rel.EndByte},
			NewText: newLeaf,
		})
	}
	return edits
}
