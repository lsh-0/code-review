package feedback

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// the import document schema, version 1. It is the contract producers code against,
// and the same bytes `code-review schema` prints.
//
//go:embed import-document.v1.schema.json
var schemaV1 []byte

// the `$id` of the version 1 schema. The compiler resolves the schema under this URL,
// so no reference in it is ever fetched from the network.
const schemaV1ID = "https://github.com/lsh-0/code-review/schemas/import-document.v1.schema.json"

// returns the embedded version 1 schema, byte-identical to the committed file.
func Schema() []byte {
	return bytes.Clone(schemaV1)
}

// the compiled version 1 schema, built once. Format assertion is on, so a timestamp
// that is not RFC 3339 is a violation and not an annotation.
var compiledV1 = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaV1))
	if err != nil {
		return nil, fmt.Errorf("embedded schema is not JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	if err := compiler.AddResource(schemaV1ID, doc); err != nil {
		return nil, fmt.Errorf("embedded schema cannot be added: %w", err)
	}
	return compiler.Compile(schemaV1ID)
})

// one way a document breaks the schema: where it is, as a JSON Pointer into the
// document, and what was wrong there.
type Violation struct {
	Pointer string
	Message string
}

func (v Violation) String() string {
	pointer := v.Pointer
	if pointer == "" {
		pointer = "/"
	}
	return pointer + ": " + v.Message
}

// every violation found in one document, sorted by pointer and then by message.
type SchemaError struct {
	Violations []Violation
}

func (e *SchemaError) Error() string {
	lines := make([]string, 0, len(e.Violations))
	for _, violation := range e.Violations {
		lines = append(lines, violation.String())
	}
	return "document does not match the import schema:\n  " + strings.Join(lines, "\n  ")
}

// validates `instance`, a value produced by `jsonschema.UnmarshalJSON`, against the
// version 1 schema. Returns a `*SchemaError` naming every violation, or nil.
func validateV1(instance any) error {
	schema, err := compiledV1()
	if err != nil {
		return err
	}
	err = schema.Validate(instance)
	if err == nil {
		return nil
	}
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) {
		return err
	}
	return &SchemaError{Violations: violations(validation)}
}

// the message for a property the schema declares but the object's tag forbids, such
// as `lines` on a whole-file position. The validator renders it as "false schema",
// which names the mechanism and not the problem.
const notAllowedMessage = "property is not allowed here"

// flattens a validation error tree to its leaf causes. An inner node only says that
// some cause below it failed, so it carries nothing a producer can act on.

func violations(root *jsonschema.ValidationError) []Violation {
	found := make([]Violation, 0)
	seen := make(map[Violation]bool)
	var walk func(node *jsonschema.ValidationError)
	walk = func(node *jsonschema.ValidationError) {
		if len(node.Causes) > 0 {
			for _, cause := range node.Causes {
				walk(cause)
			}
			return
		}
		message := node.ErrorKind.LocalizedString(messagePrinter)
		if _, ok := node.ErrorKind.(*kind.FalseSchema); ok {
			message = notAllowedMessage
		}
		violation := Violation{Pointer: pointer(node.InstanceLocation), Message: message}
		if !seen[violation] {
			seen[violation] = true
			found = append(found, violation)
		}
	}
	walk(root)
	sort.Slice(found, func(i, j int) bool {
		if found[i].Pointer != found[j].Pointer {
			return found[i].Pointer < found[j].Pointer
		}
		return found[i].Message < found[j].Message
	})
	return found
}

// a JSON Pointer (RFC 6901) for a path of reference tokens.
func pointer(tokens []string) string {
	var builder strings.Builder
	for _, token := range tokens {
		builder.WriteByte('/')
		token = strings.ReplaceAll(token, "~", "~0")
		builder.WriteString(strings.ReplaceAll(token, "/", "~1"))
	}
	return builder.String()
}
