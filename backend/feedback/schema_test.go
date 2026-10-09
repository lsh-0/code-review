package feedback

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	ft "code-review/feedback/feedbacktest"
)

// a fixture document with `edit` applied to its decoded JSON, so a test states only
// the one thing it changes.
func editedFixture(t *testing.T, edit func(doc map[string]any)) []byte {
	t.Helper()
	data, err := os.ReadFile(ft.Fixture1430)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	edited, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return edited
}

// the first file-feedback thread of a decoded fixture document.
func firstFileThread(doc map[string]any) map[string]any {
	return doc["file_feedback"].([]any)[0].(map[string]any)
}

// the root comment of the first file-feedback thread.
func firstFileRoot(doc map[string]any) map[string]any {
	return firstFileThread(doc)["root"].(map[string]any)
}

// decodes `given` and returns the schema violations, failing when it decodes or fails
// for a reason other than the schema.
func violationsOf(t *testing.T, given []byte) []Violation {
	t.Helper()
	_, err := Decode(given)
	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("expected a schema error, got %v", err)
	}
	return schemaErr.Violations
}

// reports whether `found` has a violation at `at` whose message contains `text`.
func hasViolation(found []Violation, at string, text string) bool {
	for _, violation := range found {
		if violation.Pointer == at && strings.Contains(violation.Message, text) {
			return true
		}
	}
	return false
}

func TestSchemaCompiles(t *testing.T) {
	if _, err := compiledV1(); err != nil {
		t.Fatalf("embedded schema does not compile: %v", err)
	}
}

// `Schema` must return the committed file byte for byte, since that is what
// `code-review schema` prints and what producers read from the repository.
func TestSchemaIsTheCommittedFile(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	expected, err := os.ReadFile(filepath.Join(filepath.Dir(here), "import-document.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	actual := Schema()
	if !bytes.Equal(actual, expected) {
		t.Error("Schema() differs from the committed schema file")
	}
}

// a caller modifying the returned bytes must not change what the next caller gets.
func TestSchemaReturnsACopy(t *testing.T) {
	given := Schema()
	given[0] = 'X'
	if Schema()[0] == 'X' {
		t.Error("Schema() returned the embedded bytes, not a copy")
	}
}

// the schema states the one rule it cannot enforce, so a producer reading it learns
// the rule without reading the importer.
func TestSchemaStatesTheSpanOrderingRule(t *testing.T) {
	var schema struct {
		Defs map[string]struct {
			Description string `json:"description"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatal(err)
	}
	actual := schema.Defs["span"].Description
	if !strings.Contains(actual, "`first` does not exceed `last`") {
		t.Errorf("span description does not state the ordering rule: %q", actual)
	}
}

func TestSchemaRejectsUnknownProperties(t *testing.T) {
	given := []struct {
		name  string
		edit  func(doc map[string]any)
		at    string
		named string
	}{
		{
			name:  "top level",
			edit:  func(doc map[string]any) { doc["file_feedbacks"] = []any{} },
			at:    "",
			named: "'file_feedbacks'",
		},
		{
			name:  "in a comment",
			edit:  func(doc map[string]any) { firstFileRoot(doc)["severity"] = "high" },
			at:    "/file_feedback/0/root",
			named: "'severity'",
		},
		{
			name: "in a nested reply",
			edit: func(doc map[string]any) {
				reply := map[string]any{}
				maps.Copy(reply, firstFileRoot(doc))
				reply["id"] = 999
				reply["replies"] = []any{}
				reply["mood"] = "terse"
				firstFileRoot(doc)["replies"] = []any{reply}
			},
			at:    "/file_feedback/0/root/replies/0",
			named: "'mood'",
		},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			actual := violationsOf(t, editedFixture(t, test.edit))
			if !hasViolation(actual, test.at, test.named) {
				t.Errorf("expected a violation at %q naming %s, got %v", test.at, test.named, actual)
			}
		})
	}
}

// a property that belongs to another tag is not allowed: `lines` means nothing on a
// whole-file position.
func TestSchemaRejectsAPropertyOfAnotherTag(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileThread(doc)["location"].(map[string]any)["position"] = map[string]any{
			"kind": "whole_file", "lines": map[string]any{"first": 1, "last": 1},
		}
	})

	actual := violationsOf(t, given)
	if !hasViolation(actual, "/file_feedback/0/location/position/lines", notAllowedMessage) {
		t.Errorf("expected `lines` on a whole-file position to be rejected, got %v", actual)
	}
}

func TestSchemaRequiresEachTagsProperties(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileThread(doc)["location"].(map[string]any)["position"] = map[string]any{
			"kind": "unchanged", "original_lines": map[string]any{"first": 1, "last": 1},
		}
	})

	actual := violationsOf(t, given)
	if !hasViolation(actual, "/file_feedback/0/location/position", "'updated_lines'") {
		t.Errorf("expected missing `updated_lines` to be named, got %v", actual)
	}
}

func TestSchemaAcceptsAWholeFilePositionWithNoSpan(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileThread(doc)["location"].(map[string]any)["position"] = map[string]any{"kind": "whole_file"}
	})

	if _, err := Decode(given); err != nil {
		t.Errorf("expected a whole-file position to decode, got %v", err)
	}
}

// an unknown tag reports the tag with its allowed values, and not each property the
// tag would have allowed.
func TestSchemaReportsAnUnknownTagOnce(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileThread(doc)["location"].(map[string]any)["position"].(map[string]any)["kind"] = "sideways"
	})

	actual := violationsOf(t, given)
	expected := []Violation{{
		Pointer: "/file_feedback/0/location/position/kind",
		Message: "value must be one of 'whole_file', 'original', 'updated', 'unchanged'",
	}}
	if len(actual) != 1 || actual[0] != expected[0] {
		t.Errorf("got %v, want %v", actual, expected)
	}
}

func TestSchemaVersion(t *testing.T) {
	given := []struct {
		name    string
		version any
		accept  bool
		message string
	}{
		{name: "absent", version: nil, accept: true},
		{name: "one", version: 1, accept: true},
		{name: "two", version: 2, accept: false, message: "unsupported schema_version 2: supported versions are 1"},
		{name: "not a number", version: "1", accept: false, message: "/schema_version: value must be 1"},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			document := editedFixture(t, func(doc map[string]any) {
				if test.version != nil {
					doc["schema_version"] = test.version
				}
			})
			_, err := Decode(document)
			if test.accept && err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
			if !test.accept && (err == nil || !strings.Contains(err.Error(), test.message)) {
				t.Errorf("expected an error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestSchemaNamesEveryIndependentViolation(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileRoot(doc)["author"].(map[string]any)["kind"] = "robot"
		doc["discussion"].([]any)[0].(map[string]any)["resolution"].(map[string]any)["state"] = "half_done"
	})

	actual := violationsOf(t, given)
	if !hasViolation(actual, "/file_feedback/0/root/author/kind", "value must be one of") {
		t.Errorf("expected the author kind to be named, got %v", actual)
	}
	if !hasViolation(actual, "/discussion/0/resolution/state", "value must be one of") {
		t.Errorf("expected the resolution state to be named, got %v", actual)
	}
}

// the schema cannot compare two properties, so an inverted span passes it and is
// rejected by the decoder with its location and both values.
func TestInvertedSpanPassesTheSchemaAndIsRejected(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileThread(doc)["location"].(map[string]any)["position"] = map[string]any{
			"kind": "updated", "lines": map[string]any{"first": 20, "last": 12},
		}
	})

	instance := map[string]any{}
	if err := json.Unmarshal(given, &instance); err != nil {
		t.Fatal(err)
	}

	_, err := Decode(given)
	var schemaErr *SchemaError
	if errors.As(err, &schemaErr) {
		t.Fatalf("expected the schema to accept an inverted span, got %v", err)
	}
	expected := "/file_feedback/0/location/position/lines: span first 20 exceeds last 12"
	if err == nil || err.Error() != expected {
		t.Errorf("got %v, want %q", err, expected)
	}
}

func TestSchemaAcceptsNullOptionals(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		pullRequest := doc["pull_request"].(map[string]any)
		pullRequest["description"] = nil
		pullRequest["updated_at"] = nil
		pullRequest["source"].(map[string]any)["commit"] = nil
		firstFileRoot(doc)["updated_at"] = nil
		firstFileRoot(doc)["author"].(map[string]any)["uuid"] = nil
	})

	if _, err := Decode(given); err != nil {
		t.Errorf("expected null optionals to decode, got %v", err)
	}
}

func TestSchemaRejectsATimestampThatIsNotRFC3339(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		firstFileRoot(doc)["created_at"] = "yesterday"
	})

	actual := violationsOf(t, given)
	if !hasViolation(actual, "/file_feedback/0/root/created_at", "date-time") {
		t.Errorf("expected the timestamp to be named as not a date-time, got %v", actual)
	}
}

// the shortest document a producer can write: optionals with safe defaults omitted.
func TestSchemaAcceptsAMinimalDocument(t *testing.T) {
	given := `{
		"pull_request": {
			"id": 7, "title": "t", "state": "open",
			"author": {"display_name": "a", "kind": "human"},
			"created_at": "2026-01-01T00:00:00Z",
			"source": {"branch": "f"}, "destination": {"branch": "m"}
		},
		"discussion": [],
		"file_feedback": [{
			"location": {"kind": "file_revision", "path": "a.go", "position": {"kind": "whole_file"}},
			"resolution": {"state": "unresolved"},
			"root": {"id": 1, "author": {"display_name": "a", "kind": "human"},
			         "body": {"kind": "markdown", "text": "x"}, "created_at": "2026-01-01T00:00:00Z"}
		}]
	}`

	if _, err := Decode([]byte(given)); err != nil {
		t.Errorf("expected a minimal document to decode, got %v", err)
	}
}

func TestPointerEscapesReservedCharacters(t *testing.T) {
	given := []string{"a/b", "c~d"}
	expected := "/a~1b/c~0d"
	actual := pointer(given)
	if actual != expected {
		t.Errorf("got %q, want %q", actual, expected)
	}
}

// a misspelt property is reported even when its object has another violation, so a
// producer fixing one mistake is not left to discover the next on the following run.
func TestSchemaReportsAMisspeltPropertyBesideABadTag(t *testing.T) {
	given := editedFixture(t, func(doc map[string]any) {
		author := firstFileRoot(doc)["author"].(map[string]any)
		author["kind"] = "robot"
		author["nick"] = "x"
	})

	actual := violationsOf(t, given)
	if !hasViolation(actual, "/file_feedback/0/root/author", "'nick'") {
		t.Errorf("expected the misspelt property to be named, got %v", actual)
	}
	if !hasViolation(actual, "/file_feedback/0/root/author/kind", "value must be one of") {
		t.Errorf("expected the bad tag to be named, got %v", actual)
	}
}
