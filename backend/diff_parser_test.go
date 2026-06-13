package main

import (
	"strings"
	"testing"
)

// a diff with two hunks separated by a large unchanged gap: the second hunk
// header must be parsed as a header, never captured as a content line. This
// guards against the second `@@` leaking into a DiffLine's content.
func TestParseDiffTwoHunksNoHeaderLeak(t *testing.T) {
	given := "diff --git a/cmd/adl/main.go b/cmd/adl/main.go\n" +
		"index 840dbde..cbf92ad 100644\n" +
		"--- a/cmd/adl/main.go\n" +
		"+++ b/cmd/adl/main.go\n" +
		"@@ -21,6 +21,7 @@ import (\n" +
		" \t\"adl/common\"\n" +
		" \t\"adl/common/oidc\"\n" +
		" \tmanagement_app \"adl/management/app\"\n" +
		"+\tproduction_app \"adl/production/app\"\n" +
		" \treview_app \"adl/review/app\"\n" +
		" \tsubmit_app \"adl/submit/app\"\n" +
		" \n" +
		"@@ -91,6 +92,10 @@ func build_handler(deps handler_deps) (http.Handler, error) {\n" +
		" \troot.Mount(\"/review\", review_handler)\n" +
		" \n" +
		"+\t// production API routes\n" +
		" \t// top-level health check\n"

	files := ParseDiff(given)

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(files[0].Hunks))
	}

	for hi, hunk := range files[0].Hunks {
		for _, line := range hunk.Lines {
			if strings.Contains(line.Content, "@@") {
				t.Errorf("hunk %d: header leaked into a content line: %q", hi, line.Content)
			}
		}
	}

	second := files[0].Hunks[1]
	if second.NewStart != 92 || second.OldStart != 91 {
		t.Errorf("expected second hunk to start at old 91 / new 92, got old %d / new %d",
			second.OldStart, second.NewStart)
	}
}

func TestParseDiff(t *testing.T) {
	diffText := `diff --git a/file1.go b/file1.go
index abc123..def456 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {
diff --git a/file2.go b/file2.go
index 111222..333444 100644
--- a/file2.go
+++ b/file2.go
@@ -10,5 +10,6 @@ func test() {
 	x := 1
 	y := 2
+	z := 3
 	return
 }
`

	files := ParseDiff(diffText)

	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}

	if files[0].Path != "file1.go" {
		t.Errorf("Expected first file to be 'file1.go', got '%s'", files[0].Path)
	}

	if files[1].Path != "file2.go" {
		t.Errorf("Expected second file to be 'file2.go', got '%s'", files[1].Path)
	}

	if len(files[0].Hunks) != 1 {
		t.Errorf("Expected 1 hunk for file1.go, got %d", len(files[0].Hunks))
	}

	if len(files[1].Hunks) != 1 {
		t.Errorf("Expected 1 hunk for file2.go, got %d", len(files[1].Hunks))
	}
}

func TestParseDiffHunk(t *testing.T) {
	diffText := `diff --git a/test.go b/test.go
index abc123..def456 100644
--- a/test.go
+++ b/test.go
@@ -5,6 +5,7 @@ func example() {
 	a := 1
 	b := 2
 	c := 3
+	d := 4
 	return
 }
`

	files := ParseDiff(diffText)

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	file := files[0]
	if len(file.Hunks) != 1 {
		t.Fatalf("Expected 1 hunk, got %d", len(file.Hunks))
	}

	hunk := file.Hunks[0]
	if hunk.OldStart != 5 {
		t.Errorf("Expected old start line 5, got %d", hunk.OldStart)
	}

	if hunk.OldLines != 6 {
		t.Errorf("Expected 6 old lines, got %d", hunk.OldLines)
	}

	if hunk.NewStart != 5 {
		t.Errorf("Expected new start line 5, got %d", hunk.NewStart)
	}

	if hunk.NewLines != 7 {
		t.Errorf("Expected 7 new lines, got %d", hunk.NewLines)
	}
}

func TestParseDiffLines(t *testing.T) {
	diffText := `diff --git a/test.go b/test.go
index abc123..def456 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,4 @@
 context line 1
+added line
-removed line
 context line 2
`

	files := ParseDiff(diffText)

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	hunk := files[0].Hunks[0]
	if len(hunk.Lines) != 4 {
		t.Fatalf("Expected 4 lines, got %d", len(hunk.Lines))
	}

	if hunk.Lines[0].Type != LineContext {
		t.Errorf("Expected first line to be context, got %v", hunk.Lines[0].Type)
	}

	if hunk.Lines[1].Type != LineAdded {
		t.Errorf("Expected second line to be added, got %v", hunk.Lines[1].Type)
	}

	if hunk.Lines[2].Type != LineRemoved {
		t.Errorf("Expected third line to be removed, got %v", hunk.Lines[2].Type)
	}

	if hunk.Lines[3].Type != LineContext {
		t.Errorf("Expected fourth line to be context, got %v", hunk.Lines[3].Type)
	}

	if hunk.Lines[0].Content != "context line 1" {
		t.Errorf("Expected 'context line 1', got '%s'", hunk.Lines[0].Content)
	}

	if hunk.Lines[1].Content != "added line" {
		t.Errorf("Expected 'added line', got '%s'", hunk.Lines[1].Content)
	}

	if hunk.Lines[2].Content != "removed line" {
		t.Errorf("Expected 'removed line', got '%s'", hunk.Lines[2].Content)
	}
}

func TestParseDiffBinaryFile(t *testing.T) {
	given := `diff --git a/text.go b/text.go
index abc123..def456 100644
--- a/text.go
+++ b/text.go
@@ -1,2 +1,3 @@
 package main
+import "fmt"
 func main() {
diff --git a/image.png b/image.png
index 0de32cd..e2d5c72 100644
Binary files a/image.png and b/image.png differ
`

	files := ParseDiff(given)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	text := files[0]
	if text.Binary {
		t.Errorf("expected text.go not to be binary")
	}
	if len(text.Hunks) != 1 {
		t.Errorf("expected text.go to have 1 hunk, got %d", len(text.Hunks))
	}

	binary := files[1]
	if binary.Path != "image.png" {
		t.Errorf("expected second file 'image.png', got %q", binary.Path)
	}
	if !binary.Binary {
		t.Errorf("expected image.png to be flagged binary")
	}
	if len(binary.Hunks) != 0 {
		t.Errorf("expected image.png to have 0 hunks, got %d", len(binary.Hunks))
	}
}

func TestParseDiffEmptyInput(t *testing.T) {
	files := ParseDiff("")
	if len(files) != 0 {
		t.Errorf("Expected 0 files for empty input, got %d", len(files))
	}
}

func TestParseDiffNoChanges(t *testing.T) {
	diffText := `diff --git a/test.go b/test.go
index abc123..abc123 100644
--- a/test.go
+++ b/test.go
`

	files := ParseDiff(diffText)

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	if len(files[0].Hunks) != 0 {
		t.Errorf("Expected 0 hunks for file with no changes, got %d", len(files[0].Hunks))
	}
}
