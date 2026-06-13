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
