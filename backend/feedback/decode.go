package feedback

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// renders schema violation messages. English only: the messages reach a terminal or
// the GUI banner, both of which are English throughout.
var messagePrinter = message.NewPrinter(language.English)

// decodes and validates an import document.
//
// The steps run in this order, and the first that fails returns:
//  1. parse as JSON.
//  2. check `schema_version`, so an unsupported version reports itself and not a
//     list of schema violations.
//  3. validate against the schema for that version, naming every violation.
//  4. decode into `Document`.
//  5. check that no span's `first` exceeds its `last`, which the schema cannot
//     express.
//
// A document that passes every step is safe for the mapping to read without
// re-checking any tag or span.
func Decode(data []byte) (*Document, error) {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("document is not valid JSON: %w", err)
	}

	if err := checkVersion(instance); err != nil {
		return nil, err
	}

	if err := validateV1(instance); err != nil {
		return nil, err
	}

	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		// unreachable for a schema-valid document unless the schema and the types
		// disagree, which is a defect here and not in the document.
		return nil, fmt.Errorf("document matches the schema but does not decode: %w", err)
	}

	if err := checkSpans(doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// rejects a `schema_version` other than the one this importer supports. Leaves a
// non-object, or a version that is not a number, for the schema to report.
func checkVersion(instance any) error {
	object, ok := instance.(map[string]any)
	if !ok {
		return nil
	}
	raw, present := object["schema_version"]
	if !present {
		return nil
	}
	number, ok := raw.(json.Number)
	if !ok {
		return nil
	}
	if number.String() == fmt.Sprint(SchemaVersion) {
		return nil
	}
	return fmt.Errorf("unsupported schema_version %s: supported versions are %d", number, SchemaVersion)
}

// rejects a span whose `first` exceeds its `last`, naming its location in the
// document. The schema states this rule in the span's description and cannot
// enforce it.
func checkSpans(doc Document) error {
	collections := []struct {
		name    string
		threads []Thread
	}{
		{"discussion", doc.Discussion},
		{"file_feedback", doc.FileFeedback},
	}
	for _, collection := range collections {
		for i, thread := range collection.threads {
			if thread.Location.Kind != LocationFileRevision {
				continue
			}
			base := fmt.Sprintf("/%s/%d/location/position", collection.name, i)
			position := thread.Location.Position
			spans := map[string]Span{}
			switch position.Kind {
			case PositionOriginal, PositionUpdated:
				spans["lines"] = position.Lines
			case PositionUnchanged:
				spans["original_lines"] = position.OriginalLines
				spans["updated_lines"] = position.UpdatedLines
			}
			for _, name := range []string{"lines", "original_lines", "updated_lines"} {
				span, present := spans[name]
				if present && !span.Valid() {
					return fmt.Errorf("%s/%s: span first %d exceeds last %d", base, name, span.First, span.Last)
				}
			}
		}
	}
	return nil
}

// encodes a document in the upstream layout: optionals as `null`, absent lists as
// `[]`, and only the fields each tag makes meaningful. Indented by two spaces with a
// trailing newline.
func Encode(doc Document) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
