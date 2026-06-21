package main

import (
	"code-review/model"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// the directory the TypeScript wire-type tests read fixtures from. The Go side
// is the source of truth for the bridge JSON shape; the fixtures hold real
// marshalled output so the TS decode (ts/client_test.ts) is pinned against what
// the backend actually produces, and a Go-side shape change fails here rather
// than silently at runtime in the webview.
const tsFixtureDir = "../ts/testdata"

// the environment variable that, when set during test setup, refreshes the
// committed fixtures instead of asserting against them.
const writeFixturesEnv = "CODE_REVIEW_WRITE_FIXTURES"

// build the JSON the bound `App` methods return, keyed by fixture filename. No
// `*testing.T` dependency and no environment reads, so both the assertion test
// and the setup-time writer can call it.
func wireFixtures() (map[string]string, error) {
	tmpDir, err := os.MkdirTemp("", "wire-fixtures-data")
	if err != nil {
		return nil, fmt.Errorf("temp data dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	repoDir, err := os.MkdirTemp("", "wire-fixtures-repo")
	if err != nil {
		return nil, fmt.Errorf("temp repo dir: %w", err)
	}
	defer os.RemoveAll(repoDir)

	app := &App{
		review:    model.NewReview(repoDir, "feature", "main"),
		repoPath:  repoDir,
		dataDir:   tmpDir,
		statePath: filepath.Join(tmpDir, "state.json"),
		userName:  "Test User",
	}

	// a comment mutation result, with one active comment carrying context.
	mutationRaw, err := app.AddComment("a.go", "a note", 7, "before", "the line", "after")
	if err != nil {
		return nil, fmt.Errorf("AddComment: %w", err)
	}

	// a reply makes the thread multi-entry, exercising parent_id on the wire.
	var added CommentMutationResult
	if err := json.Unmarshal([]byte(mutationRaw), &added); err != nil {
		return nil, fmt.Errorf("unmarshal mutation: %w", err)
	}
	replyRaw, err := app.AddReply("a.go", added.Comments[0].ID, "a reply")
	if err != nil {
		return nil, fmt.Errorf("AddReply: %w", err)
	}

	// review info, marshalled the same way `GetReviewInfo` does.
	infoRaw, err := app.GetReviewInfo()
	if err != nil {
		return nil, fmt.Errorf("GetReviewInfo: %w", err)
	}

	// commented files: the overview's per-file feedback shape. `GetCommentedFiles`
	// derives its list from the parsed git diff, which is empty without a repo,
	// so the shape is marshalled here from the same struct it uses — grounding
	// the fixture in the real type rather than faking the JSON.
	commentedFiles := []struct {
		Path     string           `json:"path"`
		Comments []*model.Comment `json:"comments"`
	}{
		{
			Path: "a.go",
			Comments: []*model.Comment{
				model.NewCommentWithContext("a note", 7, "Test User", "before", "the line", "after"),
			},
		},
	}
	commentedBytes, err := json.Marshal(commentedFiles)
	if err != nil {
		return nil, fmt.Errorf("marshal commented files: %w", err)
	}

	return map[string]string{
		"mutation_result.json": replyRaw,
		"review_info.json":     infoRaw,
		"commented_files.json": string(commentedBytes),
	}, nil
}

// write the fixtures to `tsFixtureDir`, pretty-printed. Used by the setup-time
// refresh path, not by an assertion test.
func writeWireFixtures(fixtures map[string]string) error {
	if err := os.MkdirAll(tsFixtureDir, 0o755); err != nil {
		return fmt.Errorf("mkdir fixtures: %w", err)
	}
	for name, raw := range fixtures {
		pretty, err := indentJSON(raw)
		if err != nil {
			return fmt.Errorf("indent %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tsFixtureDir, name), pretty, 0o644); err != nil {
			return fmt.Errorf("write fixture %s: %w", name, err)
		}
	}
	return nil
}

// assert the committed fixtures still carry the same wire *shape* as the current
// output. The comparison is on the recursive set of JSON keys (and array element
// shapes), not values — so random comment IDs do not make it flap, while a
// renamed, added, or removed field (the actual drift the TS decode cares about)
// fails it. The env-gated refresh happens in `TestMain`, so this test only ever
// asserts: no environment read, no write side effect, no branching.
func TestWireFixtures(t *testing.T) {
	fixtures, err := wireFixtures()
	if err != nil {
		t.Fatalf("build wire fixtures: %v", err)
	}
	for name, raw := range fixtures {
		committed, err := os.ReadFile(filepath.Join(tsFixtureDir, name))
		if err != nil {
			t.Fatalf("read fixture %s (run with %s=1 to create): %v", name, writeFixturesEnv, err)
		}
		if got, want := jsonShape(t, raw), jsonShape(t, string(committed)); got != want {
			t.Errorf("fixture %s shape drifted: backend now produces %s, committed is %s; "+
				"re-run with %s=1 and update ts/client_test.ts to match", name, got, want, writeFixturesEnv)
		}
	}
}

// a stable, value-independent description of a JSON document's shape: the sorted
// key set at each object level and the shape of array elements, recursively.
// Two documents share a shape when they have the same fields in the same nested
// positions, regardless of the scalar values.
func jsonShape(t *testing.T, raw string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("jsonShape: unmarshal: %v", err)
	}
	return shapeOf(v)
}

func shapeOf(v any) string {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ":" + shapeOf(val[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		// union the element shapes so a heterogeneous array (e.g. a root comment
		// plus a reply, which carries the extra parent_id) reports both shapes.
		seen := map[string]bool{}
		shapes := []string{}
		for _, e := range val {
			s := shapeOf(e)
			if !seen[s] {
				seen[s] = true
				shapes = append(shapes, s)
			}
		}
		sort.Strings(shapes)
		return "[" + strings.Join(shapes, "|") + "]"
	default:
		return "scalar"
	}
}

// pretty-print a raw JSON string with two-space indentation and a trailing
// newline, matching the committed fixture format.
func indentJSON(raw string) ([]byte, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return append(out, '\n'), nil
}

// the package's test entry point. When `writeFixturesEnv` is set it refreshes
// the committed fixtures as a one-shot setup step and exits without running the
// suite, keeping the environment read and the write side effect out of any test
// body. Otherwise it runs the tests normally.
func TestMain(m *testing.M) {
	if os.Getenv(writeFixturesEnv) == "1" {
		fixtures, err := wireFixtures()
		if err != nil {
			log.Fatalf("build wire fixtures: %v", err)
		}
		if err := writeWireFixtures(fixtures); err != nil {
			log.Fatalf("write wire fixtures: %v", err)
		}
		log.Printf("wrote %d fixtures to %s", len(fixtures), tsFixtureDir)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
