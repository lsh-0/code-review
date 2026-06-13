package main

import (
	"fmt"
	"strings"
)

// a per-session cache of full file bodies keyed by `rev` then `path`, so
// repeated context expansions of the same file do not re-shell-out to git.
// The fetch function is injected to keep the cache testable without git.
type fileContentCache struct {
	bodies map[string]map[string]string
	fetch  func(rev, path string) (string, error)
}

func newFileContentCache(fetch func(rev, path string) (string, error)) *fileContentCache {
	return &fileContentCache{
		bodies: map[string]map[string]string{},
		fetch:  fetch,
	}
}

// return the cached body for `(rev, path)`, fetching and storing it on first
// request. A fetch error is returned and nothing is cached.
func (c *fileContentCache) get(rev, path string) (string, error) {
	if byPath, ok := c.bodies[rev]; ok {
		if body, ok := byPath[path]; ok {
			return body, nil
		}
	}

	body, err := c.fetch(rev, path)
	if err != nil {
		return "", err
	}

	if c.bodies[rev] == nil {
		c.bodies[rev] = map[string]string{}
	}
	c.bodies[rev][path] = body
	return body, nil
}

// split a file body into its lines, dropping a single trailing newline so a
// file ending in "\n" does not yield a spurious empty final line. The result
// is 0-indexed; line N of the file is element N-1.
func splitLines(body string) []string {
	if body == "" {
		return []string{}
	}
	trimmed := strings.TrimSuffix(body, "\n")
	return strings.Split(trimmed, "\n")
}

// count the lines in a file body.
func lineCount(body string) int {
	return len(splitLines(body))
}

// build context `DiffLine`s for the inclusive new-file line range
// [startNew, endNew] from a file body. `oldOffset` is added to each new line
// number to derive the old line number (old = new + oldOffset), constant
// across a contiguous unchanged run. The range is clamped to the file's
// bounds; an empty range yields no lines.
func contextLines(body string, startNew, endNew, oldOffset int) []DiffLine {
	all := splitLines(body)
	total := len(all)

	if startNew < 1 {
		startNew = 1
	}
	if endNew > total {
		endNew = total
	}

	lines := []DiffLine{}
	for n := startNew; n <= endNew; n++ {
		lines = append(lines, DiffLine{
			Type:      LineContext,
			Content:   all[n-1],
			OldLineNo: n + oldOffset,
			NewLineNo: n,
		})
	}
	return lines
}

// validate a requested range and return its context lines, or an error when
// the range is malformed.
func fileLineRange(body string, startNew, endNew, oldOffset int) ([]DiffLine, error) {
	if startNew > endNew {
		return nil, fmt.Errorf("invalid range: start %d after end %d", startNew, endNew)
	}
	return contextLines(body, startNew, endNew, oldOffset), nil
}
