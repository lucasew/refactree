package lint

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/version"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// WriteSARIF writes a SARIF 2.1.0 log for the lint result to w.
// Findings with successful site edits always include result.fixes.
func WriteSARIF(w io.Writer, root string, res Result) error {
	rules := make([]sarifReportingDescriptor, 0, len(res.Rules))
	ruleIndex := map[string]int{}
	for i, cr := range res.Rules {
		ruleIndex[cr.Spec.ID] = i
		rules = append(rules, sarifReportingDescriptor{
			ID:               cr.Spec.ID,
			Name:             cr.Spec.ID,
			ShortDescription: sarifMessage{Text: cr.Spec.Message},
			FullDescription:  sarifMessage{Text: cr.Spec.Message},
			DefaultConfiguration: &sarifReportingConfiguration{
				Level: cr.Level,
			},
		})
	}

	results := make([]sarifResult, 0, len(res.Findings))
	for _, f := range res.Findings {
		idx, ok := ruleIndex[f.RuleID]
		if !ok {
			idx = -1
		}
		uri := toSARIFURI(f.File)
		r := sarifResult{
			RuleID:    f.RuleID,
			RuleIndex: idx,
			Level:     f.Level,
			Message:   sarifMessage{Text: f.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: uri},
					Region: sarifRegion{
						StartLine:   f.Line,
						StartColumn: f.Column,
						EndLine:     f.EndLine,
						EndColumn:   f.EndCol,
						Snippet:     &sarifArtifactContent{Text: f.Snippet},
					},
				},
			}},
		}
		if f.Fixable && !f.FixSkipped && len(f.SiteEdits) > 0 {
			fix, err := editsToSARIFFix(root, f.Message, f.SiteEdits, f.source)
			if err != nil {
				return err
			}
			r.Fixes = []sarifFix{fix}
		}
		results = append(results, r)
	}

	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifToolComponent{
					Name:           "refactree",
					Version:        version.GetBuildID(),
					InformationURI: "https://github.com/lucasew/refactree",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func editsToSARIFFix(root, description string, edits []ingest.Edit, source []byte) (sarifFix, error) {
	byFile := map[string][]ingest.Edit{}
	order := []string{}
	for _, e := range edits {
		if _, ok := byFile[e.File]; !ok {
			order = append(order, e.File)
		}
		byFile[e.File] = append(byFile[e.File], e)
	}
	changes := make([]sarifArtifactChange, 0, len(order))
	for _, file := range order {
		fileEdits := byFile[file]
		src := source
		if src == nil {
			abs := file
			if root != "" && !filepath.IsAbs(file) {
				abs = filepath.Join(root, filepath.FromSlash(file))
			}
			b, err := os.ReadFile(abs)
			if err != nil {
				return sarifFix{}, err
			}
			src = b
		}
		li := grammar.NewLineIndexBytes(src)
		repls := make([]sarifReplacement, 0, len(fileEdits))
		for _, e := range fileEdits {
			sl, sc0 := li.LineColumnAtU32(e.StartByte)
			el, ec0 := li.LineColumnAtU32(e.EndByte)
			repls = append(repls, sarifReplacement{
				DeletedRegion: sarifRegion{
					StartLine:   sl,
					StartColumn: sc0 + 1,
					EndLine:     el,
					EndColumn:   ec0 + 1,
				},
				InsertedContent: &sarifArtifactContent{Text: e.NewText},
			})
		}
		changes = append(changes, sarifArtifactChange{
			ArtifactLocation: sarifArtifactLocation{URI: toSARIFURI(file)},
			Replacements:     repls,
		})
	}
	return sarifFix{
		Description:     &sarifMessage{Text: description},
		ArtifactChanges: changes,
	}, nil
}

func toSARIFURI(file string) string {
	return strings.TrimPrefix(filepath.ToSlash(file), "./")
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifToolComponent `json:"driver"`
}

type sarifToolComponent struct {
	Name           string                     `json:"name"`
	Version        string                     `json:"version,omitempty"`
	InformationURI string                     `json:"informationUri,omitempty"`
	Rules          []sarifReportingDescriptor `json:"rules,omitempty"`
}

type sarifReportingDescriptor struct {
	ID                   string                       `json:"id"`
	Name                 string                       `json:"name,omitempty"`
	ShortDescription     sarifMessage                 `json:"shortDescription,omitempty"`
	FullDescription      sarifMessage                 `json:"fullDescription,omitempty"`
	DefaultConfiguration *sarifReportingConfiguration `json:"defaultConfiguration,omitempty"`
}

type sarifReportingConfiguration struct {
	Level string `json:"level,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	RuleIndex int             `json:"ruleIndex"` // 0 is valid; do not omitempty
	Level     string          `json:"level,omitempty"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
	Fixes     []sarifFix      `json:"fixes,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int                   `json:"startLine,omitempty"`
	StartColumn int                   `json:"startColumn,omitempty"`
	EndLine     int                   `json:"endLine,omitempty"`
	EndColumn   int                   `json:"endColumn,omitempty"`
	Snippet     *sarifArtifactContent `json:"snippet,omitempty"`
}

type sarifArtifactContent struct {
	Text string `json:"text,omitempty"`
}

type sarifFix struct {
	Description     *sarifMessage         `json:"description,omitempty"`
	ArtifactChanges []sarifArtifactChange `json:"artifactChanges"`
}

type sarifArtifactChange struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Replacements     []sarifReplacement    `json:"replacements"`
}

type sarifReplacement struct {
	DeletedRegion   sarifRegion           `json:"deletedRegion"`
	InsertedContent *sarifArtifactContent `json:"insertedContent,omitempty"`
}
