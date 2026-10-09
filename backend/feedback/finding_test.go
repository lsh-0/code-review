package feedback

import (
	"reflect"
	"strings"
	"testing"
)

func TestFindingString(t *testing.T) {
	given := Finding{Severity: Error, Code: ReplyCycle, Detail: "1 and 2 loop"}
	expected := "error reply_cycle: 1 and 2 loop"
	actual := given.String()
	if actual != expected {
		t.Errorf("got %q, want %q", actual, expected)
	}
}

func TestSortOrdersBySeverityThenCodeThenDetail(t *testing.T) {
	given := []Finding{
		{Warning, OrphanReply, "b"},
		{Debug, DeletedCommentsPresent, "x"},
		{Error, ReplyCycle, "y"},
		{Warning, OrphanReply, "a"},
		{Warning, DeletedWithBody, "z"},
	}
	expected := []Finding{
		{Error, ReplyCycle, "y"},
		{Warning, DeletedWithBody, "z"},
		{Warning, OrphanReply, "a"},
		{Warning, OrphanReply, "b"},
		{Debug, DeletedCommentsPresent, "x"},
	}

	actual := Sort(given)

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("got %v, want %v", actual, expected)
	}
	if given[0].Detail != "b" {
		t.Error("Sort modified its input")
	}
}

func TestMaxSeverity(t *testing.T) {
	if _, found := MaxSeverity(nil); found {
		t.Error("expected no severity for no findings")
	}
	warnings := []Finding{{Warning, OrphanReply, "x"}}
	if actual, _ := MaxSeverity(warnings); actual != Warning {
		t.Errorf("warnings only: got %v, want warning", actual)
	}
	mixed := append([]Finding{{Error, ReplyCycle, "y"}, {Debug, DeletedCommentsPresent, "z"}}, warnings...)
	if actual, _ := MaxSeverity(mixed); actual != Error {
		t.Errorf("mixed: got %v, want error", actual)
	}
}

func TestAtLeastDropsLowerSeverities(t *testing.T) {
	given := []Finding{{Debug, DeletedCommentsPresent, "z"}, {Warning, OrphanReply, "x"}, {Error, ReplyCycle, "y"}}
	expected := []Finding{{Error, ReplyCycle, "y"}, {Warning, OrphanReply, "x"}}
	actual := AtLeast(given, Warning)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("got %v, want %v", actual, expected)
	}
}

// the report reads as upstream's: findings most severe first, then a count line.
func TestReportListsFindingsThenSummary(t *testing.T) {
	given := []Finding{
		{Warning, OrphanReply, "reply 2 has no parent"},
		{Error, ReplyCycle, "1 and 2 loop"},
		{Warning, DeletedWithBody, "comment 3"},
	}

	lines := strings.Split(strings.TrimSpace(Report(given)), "\n")

	if len(lines) != 4 {
		t.Fatalf("expected three findings and a summary, got %q", lines)
	}
	if !strings.HasPrefix(lines[0], "error reply_cycle") {
		t.Errorf("most severe first: got %q", lines[0])
	}
	expected := "1 error, 2 warning"
	if lines[3] != expected {
		t.Errorf("summary: got %q, want %q", lines[3], expected)
	}
}

func TestReportIsEmptyForNoFindings(t *testing.T) {
	if actual := Report(nil); actual != "" {
		t.Errorf("got %q, want empty", actual)
	}
}
