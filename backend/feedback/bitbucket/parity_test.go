package bitbucket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"code-review/feedback"
)

// the port is checked against the program it replaces. Each case's expected
// document and findings are upstream `bb-feedback` output for the same payloads,
// generated once with:
//
//	main.exe x y 1 -q --pr-file <pr> --comments-file <comments> > <name>.expected.json
//	main.exe x y 1 --pr-file <pr> --comments-file <comments> 2>&1 >/dev/null |
//	    grep -E '^(debug|warning|error) ' > <name>.expected-findings.txt
//
// `synthetic_77` is a payload built to raise nearly every finding upstream knows,
// since the recorded payloads are clean.
var parityCases = []struct {
	name     string
	pr       string
	comments []string
}{
	{"aui_5405", "aui_5405_pr.json", []string{"aui_5405_comments.json"}},
	{"synthetic_3754", "synthetic_3754_pr.json", []string{"synthetic_3754_comments.json"}},
	{"aui_5405_paged", "aui_5405_pr.json", []string{
		"aui_5405_comments_page1.json", "aui_5405_comments_page2.json", "aui_5405_comments_page3.json",
	}},
	{"synthetic_77", "synthetic_77_pr.json", []string{"synthetic_77_comments.json"}},
}

// the comments of several recorded pages as one array, in page order, which is
// what retrieval returns for a paginated collection.
func concatenatedPages(t *testing.T, names []string) []byte {
	t.Helper()
	all := make([]json.RawMessage, 0)
	for _, name := range names {
		var page struct {
			Values []json.RawMessage `json:"values"`
		}
		if err := json.Unmarshal(readFixture(t, name), &page); err != nil {
			t.Fatal(err)
		}
		all = append(all, page.Values...)
	}
	data, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// the severity and code of each finding, as upstream reports them, sorted.
func findingKeys(findings []feedback.Finding) []string {
	keys := make([]string, 0, len(findings))
	for _, finding := range findings {
		keys = append(keys, finding.Severity.String()+" "+string(finding.Code))
	}
	sort.Strings(keys)
	return keys
}

func expectedFindingKeys(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bitbucket", name+".expected-findings.txt"))
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		key, _, _ := strings.Cut(line, ":")
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func asJSONValue(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestNormalisationMatchesUpstream(t *testing.T) {
	for _, test := range parityCases {
		t.Run(test.name, func(t *testing.T) {
			comments := readFixture(t, test.comments[0])
			if len(test.comments) > 1 {
				comments = concatenatedPages(t, test.comments)
			}

			doc, normalisationFindings, err := NormalisePayloads(readFixture(t, test.pr), comments)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := feedback.Encode(doc)
			if err != nil {
				t.Fatal(err)
			}

			expected := asJSONValue(t, readFixture(t, test.name+".expected.json"))
			if actual := asJSONValue(t, encoded); !reflect.DeepEqual(actual, expected) {
				_ = os.WriteFile(filepath.Join(t.TempDir(), "actual.json"), encoded, 0o644)
				t.Errorf("document differs from upstream output; actual:\n%s", encoded)
			}

			findings := append(normalisationFindings, feedback.Check(doc)...)
			if actual, want := findingKeys(findings), expectedFindingKeys(t, test.name); !reflect.DeepEqual(actual, want) {
				t.Errorf("findings differ from upstream:\n got  %v\n want %v", actual, want)
			}
		})
	}
}

// every golden document is a valid import document, so upstream output imports
// whichever program produced it.
func TestUpstreamOutputConformsToTheSchema(t *testing.T) {
	for _, test := range parityCases {
		if _, err := feedback.Decode(readFixture(t, test.name+".expected.json")); err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}

// the port's own output is a valid import document for every recorded payload.
func TestNormalisedOutputConformsToTheSchema(t *testing.T) {
	for _, test := range parityCases {
		comments := readFixture(t, test.comments[0])
		if len(test.comments) > 1 {
			comments = concatenatedPages(t, test.comments)
		}
		doc, _, err := NormalisePayloads(readFixture(t, test.pr), comments)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := feedback.Encode(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := feedback.Decode(encoded); err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}
