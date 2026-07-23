package ingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

type docFixtureCase struct {
	Name              string `json:"name"`
	Fixture           string `json:"fixture"`
	Reference         string `json:"reference"`
	ExpectError       bool   `json:"expect_error"`
	ExpectName        string `json:"expect_name"`
	ExpectDocContains string `json:"expect_doc_contains"`
}

type textAssertion struct {
	File string `json:"file"`
	Text string `json:"text"`
}

type renameFixtureCase struct {
	Name                   string          `json:"name"`
	Fixture                string          `json:"fixture"`
	Source                 string          `json:"source"`
	Destination            string          `json:"destination"`
	ExpectError            bool            `json:"expect_error"`
	ExpectEditCountAtLeast int             `json:"expect_edit_count_at_least"`
	ApplyEdits             bool            `json:"apply_edits"`
	Contains               []textAssertion `json:"contains"`
	NotContains            []textAssertion `json:"not_contains"`
}

func TestDocFor_ReferenceFixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "testdata", "reference")
	input, err := os.ReadFile(filepath.Join(fixtureRoot, "doc_cases.json"))
	if err != nil {
		t.Fatalf("reading doc_cases.json: %v", err)
	}

	var cases []docFixtureCase
	if err := json.Unmarshal(input, &cases); err != nil {
		t.Fatalf("parsing doc_cases.json: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			dir := filepath.Join(fixtureRoot, tc.Fixture)
			doc, err := ingest.DocFor(dir, tc.Reference)

			if tc.ExpectError {
				if err == nil {
					t.Fatalf("expected error, got doc %+v", doc)
				}
				return
			}

			if err != nil {
				t.Fatalf("doc lookup failed: %v", err)
			}
			if tc.ExpectName != "" && doc.Name != tc.ExpectName {
				t.Fatalf("unexpected name: got %q want %q", doc.Name, tc.ExpectName)
			}
			if tc.ExpectDocContains != "" && !strings.Contains(doc.DocString, tc.ExpectDocContains) {
				t.Fatalf("expected docstring to contain %q, got %q", tc.ExpectDocContains, doc.DocString)
			}
		})
	}
}

func TestRename_ReferenceFixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "testdata", "reference")
	input, err := os.ReadFile(filepath.Join(fixtureRoot, "rename_cases.json"))
	if err != nil {
		t.Fatalf("reading rename_cases.json: %v", err)
	}

	var cases []renameFixtureCase
	if err := json.Unmarshal(input, &cases); err != nil {
		t.Fatalf("parsing rename_cases.json: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			srcDir := filepath.Join(fixtureRoot, tc.Fixture)
			tmpDir := t.TempDir()
			copyDir(t, srcDir, tmpDir)

			plan, err := ingest.Rename(tmpDir, tc.Source, tc.Destination)
			if tc.ExpectError {
				if err == nil {
					t.Fatalf("expected error, got %d edits", len(plan.Edits))
				}
				return
			}
			if err != nil {
				t.Fatalf("rename failed: %v", err)
			}
			if tc.ExpectEditCountAtLeast > 0 && len(plan.Edits) < tc.ExpectEditCountAtLeast {
				t.Fatalf("expected at least %d edits, got %d", tc.ExpectEditCountAtLeast, len(plan.Edits))
			}

			if !tc.ApplyEdits {
				return
			}
			if err := ingest.ApplyPlan(tmpDir, plan); err != nil {
				t.Fatalf("apply edits failed: %v", err)
			}

			for _, check := range tc.Contains {
				content, err := os.ReadFile(filepath.Join(tmpDir, check.File))
				if err != nil {
					t.Fatalf("reading %s: %v", check.File, err)
				}
				if !strings.Contains(string(content), check.Text) {
					t.Fatalf("expected %s to contain %q, got:\n%s", check.File, check.Text, content)
				}
			}

			for _, check := range tc.NotContains {
				content, err := os.ReadFile(filepath.Join(tmpDir, check.File))
				if err != nil {
					t.Fatalf("reading %s: %v", check.File, err)
				}
				if strings.Contains(string(content), check.Text) {
					t.Fatalf("expected %s to not contain %q, got:\n%s", check.File, check.Text, content)
				}
			}
		})
	}
}
