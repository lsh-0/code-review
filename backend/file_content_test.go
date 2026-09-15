package main

import (
	"testing"
)

func TestContextLines(t *testing.T) {
	body := "line1\nline2\nline3\nline4\nline5\n"

	t.Run("range with old/new offset", func(t *testing.T) {
		// new lines 2..3, old lines run 5 ahead of new (oldOffset = 5).
		actual := contextLines(body, 2, 3, 5)

		if len(actual) != 2 {
			t.Fatalf("expected 2 lines, got %d", len(actual))
		}

		expected := []DiffLine{
			{Type: LineContext, Content: "line2", OldLineNo: 7, NewLineNo: 2},
			{Type: LineContext, Content: "line3", OldLineNo: 8, NewLineNo: 3},
		}
		for i, exp := range expected {
			if actual[i] != exp {
				t.Errorf("line %d: expected %+v, got %+v", i, exp, actual[i])
			}
		}
	})

	t.Run("range clamped at end of file", func(t *testing.T) {
		// requesting beyond the last line clamps to the final line.
		actual := contextLines(body, 4, 99, 0)

		if len(actual) != 2 {
			t.Fatalf("expected 2 lines (clamped), got %d", len(actual))
		}
		if actual[len(actual)-1].NewLineNo != 5 || actual[len(actual)-1].Content != "line5" {
			t.Errorf("expected last line 5 'line5', got %d %q",
				actual[len(actual)-1].NewLineNo, actual[len(actual)-1].Content)
		}
	})

	t.Run("range clamped at start of file", func(t *testing.T) {
		actual := contextLines(body, 0, 2, 0)
		if len(actual) != 2 {
			t.Fatalf("expected 2 lines, got %d", len(actual))
		}
		if actual[0].NewLineNo != 1 {
			t.Errorf("expected first revealed line to be 1, got %d", actual[0].NewLineNo)
		}
	})
}

func TestFileLineRangeInvalid(t *testing.T) {
	_, err := fileLineRange("a\nb\n", 5, 2, 0)
	if err == nil {
		t.Error("expected error for start after end, got nil")
	}
}

func TestLineCount(t *testing.T) {
	given := "a\nb\nc\n"
	expected := 3
	actual := lineCount(given)
	if actual != expected {
		t.Errorf("expected %d lines, got %d", expected, actual)
	}

	// a file without a trailing newline counts its final line too.
	if actual := lineCount("a\nb"); actual != 2 {
		t.Errorf("expected 2 lines for no-trailing-newline body, got %d", actual)
	}

	if actual := lineCount(""); actual != 0 {
		t.Errorf("expected 0 lines for empty body, got %d", actual)
	}
}

func TestCaptureContext(t *testing.T) {
	body := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"

	t.Run("interior line captures a full window", func(t *testing.T) {
		context, offset := captureContext(body, 5)
		expected := []string{"l2", "l3", "l4", "l5", "l6", "l7", "l8"}
		if len(context) != len(expected) {
			t.Fatalf("expected %d lines, got %d: %v", len(expected), len(context), context)
		}
		for i := range expected {
			if context[i] != expected[i] {
				t.Errorf("line %d: expected %q, got %q", i, expected[i], context[i])
			}
		}
		if offset != 3 {
			t.Errorf("expected offset 3 (the radius) for an interior line, got %d", offset)
		}
	})

	t.Run("line 1 clips the window at the start", func(t *testing.T) {
		context, offset := captureContext(body, 1)
		if len(context) != 4 || context[0] != "l1" || context[3] != "l4" {
			t.Errorf("expected window [l1..l4], got %v", context)
		}
		if offset != 0 {
			t.Errorf("expected offset 0 at the start of the file, got %d", offset)
		}
	})

	t.Run("final line clips the window at the end", func(t *testing.T) {
		context, offset := captureContext(body, 10)
		if len(context) != 4 || context[0] != "l7" || context[3] != "l10" {
			t.Errorf("expected window [l7..l10], got %v", context)
		}
		if offset != 3 {
			t.Errorf("expected offset 3 for the final line, got %d", offset)
		}
	})

	t.Run("line past the end of the file is empty", func(t *testing.T) {
		context, offset := captureContext(body, 11)
		if len(context) != 0 || offset != 0 {
			t.Errorf("expected empty window for an out-of-file line, got %v offset %d", context, offset)
		}
	})

	t.Run("line zero is empty", func(t *testing.T) {
		context, _ := captureContext(body, 0)
		if len(context) != 0 {
			t.Errorf("expected empty window for line 0, got %v", context)
		}
	})

	t.Run("single-line file", func(t *testing.T) {
		context, offset := captureContext("only\n", 1)
		if len(context) != 1 || context[0] != "only" || offset != 0 {
			t.Errorf("expected window [only] offset 0, got %v offset %d", context, offset)
		}
	})
}

func TestFileLineRefs(t *testing.T) {
	refs := fileLineRefs("a\nb\nc\n")
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	if refs[0].Content != "a" || refs[0].LineNumber != 1 {
		t.Errorf("expected first ref a:1, got %+v", refs[0])
	}
	if refs[2].Content != "c" || refs[2].LineNumber != 3 {
		t.Errorf("expected last ref c:3, got %+v", refs[2])
	}
	if len(fileLineRefs("")) != 0 {
		t.Error("expected no refs for an empty body")
	}
}

func TestFileContentCache(t *testing.T) {
	calls := 0
	cache := newFileContentCache(func(rev, path string) (string, error) {
		calls++
		return "body for " + rev + ":" + path, nil
	})

	first, err := cache.get("head", "a.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := cache.get("head", "a.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Errorf("expected identical cached body, got %q and %q", first, second)
	}
	if calls != 1 {
		t.Errorf("expected fetch called once, got %d", calls)
	}
}
