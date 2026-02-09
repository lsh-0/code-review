package main

import (
	"regexp"
	"strconv"
	"strings"
)

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

type DiffLine struct {
	Type      LineType
	Content   string
	OldLineNo int
	NewLineNo int
}

type DiffHunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []DiffLine
}

type DiffFile struct {
	Path  string
	Hunks []DiffHunk
}

var (
	fileHeaderRegex = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
	hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)
)

func ParseDiff(diffText string) []DiffFile {
	if diffText == "" {
		return []DiffFile{}
	}

	lines := strings.Split(diffText, "\n")
	files := []DiffFile{}
	var currentFile *DiffFile
	var currentHunk *DiffHunk
	oldLineNo := 0
	newLineNo := 0

	for _, line := range lines {
		if matches := fileHeaderRegex.FindStringSubmatch(line); matches != nil {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
					currentHunk = nil
				}
				files = append(files, *currentFile)
			}
			currentFile = &DiffFile{
				Path:  matches[2],
				Hunks: []DiffHunk{},
			}
		} else if currentFile != nil && hunkHeaderRegex.MatchString(line) {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}

			matches := hunkHeaderRegex.FindStringSubmatch(line)
			oldStart, _ := strconv.Atoi(matches[1])
			oldLines := 1
			if matches[2] != "" {
				oldLines, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newLines := 1
			if matches[4] != "" {
				newLines, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &DiffHunk{
				OldStart: oldStart,
				OldLines: oldLines,
				NewStart: newStart,
				NewLines: newLines,
				Lines:    []DiffLine{},
			}
			oldLineNo = oldStart
			newLineNo = newStart
		} else if currentHunk != nil && len(line) > 0 {
			firstChar := line[0]
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}

			var lineType LineType
			switch firstChar {
			case '+':
				lineType = LineAdded
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      lineType,
					Content:   content,
					OldLineNo: 0,
					NewLineNo: newLineNo,
				})
				newLineNo++
			case '-':
				lineType = LineRemoved
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      lineType,
					Content:   content,
					OldLineNo: oldLineNo,
					NewLineNo: 0,
				})
				oldLineNo++
			case ' ':
				lineType = LineContext
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      lineType,
					Content:   content,
					OldLineNo: oldLineNo,
					NewLineNo: newLineNo,
				})
				oldLineNo++
				newLineNo++
			}
		}
	}

	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		files = append(files, *currentFile)
	}

	return files
}
