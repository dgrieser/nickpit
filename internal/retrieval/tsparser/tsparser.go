package tsparser

import (
	"path/filepath"
	"sort"
	"strings"
)

// Lang identifies a parser family.
type Lang string

const (
	LangJS     Lang = "javascript"
	LangTS     Lang = "typescript"
	LangTSX    Lang = "tsx"
	LangJSX    Lang = "jsx"
	LangPython Lang = "python"
	LangRust   Lang = "rust"
)

// LangForExt maps a lowercase file extension (with dot) to its parser family;
// ok is false for unsupported extensions.
func LangForExt(ext string) (Lang, bool) {
	switch ext {
	case ".js", ".mjs", ".cjs":
		return LangJS, true
	case ".jsx":
		return LangJSX, true
	case ".ts", ".mts", ".cts":
		return LangTS, true
	case ".tsx":
		return LangTSX, true
	case ".py":
		return LangPython, true
	case ".rs":
		return LangRust, true
	}
	return "", false
}

// ParseFile parses src (selected by path's extension) and returns its IR.
// Parse errors do not fail the call: they surface as FileIR.HasError and
// per-symbol Symbol.HasError so the caller can degrade to low-confidence.
// An unsupported extension is the only error condition.
func ParseFile(path string, src []byte) (*FileIR, error) {
	lang, ok := LangForExt(strings.ToLower(filepath.Ext(path)))
	if !ok {
		return nil, &UnsupportedExtError{Path: path}
	}
	switch lang {
	case LangPython:
		return parsePython(path, src)
	case LangRust:
		return parseRust(path, src)
	default:
		return parseJS(lang, path, src)
	}
}

// UnsupportedExtError reports a file extension no parser family covers.
type UnsupportedExtError struct {
	Path string
}

func (e *UnsupportedExtError) Error() string {
	return "tsparser: unsupported file extension: " + e.Path
}

// lineIndex maps byte offsets to 1-based line numbers and slices source lines.
type lineIndex struct {
	src []byte
	// starts[i] is the byte offset of the first byte of line i+1.
	starts []int
}

func newLineIndex(src []byte) *lineIndex {
	starts := make([]int, 1, 128)
	starts[0] = 0
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{src: src, starts: starts}
}

// lineOf returns the 1-based line containing byte offset off.
func (ix *lineIndex) lineOf(off int) int {
	if off < 0 {
		return 1
	}
	if off > len(ix.src) {
		off = len(ix.src)
	}
	return sort.Search(len(ix.starts), func(i int) bool { return ix.starts[i] > off })
}

// lineCount returns the number of lines in the file.
func (ix *lineIndex) lineCount() int {
	n := len(ix.starts)
	// A trailing newline starts a final empty line; don't count it.
	if n > 1 && ix.starts[n-1] == len(ix.src) {
		return n - 1
	}
	return n
}

// slice returns the source text of the 1-based inclusive line range, without a
// trailing newline.
func (ix *lineIndex) slice(startLine, endLine int) string {
	if startLine < 1 {
		startLine = 1
	}
	if endLine > ix.lineCount() {
		endLine = ix.lineCount()
	}
	if startLine > endLine {
		return ""
	}
	start := ix.starts[startLine-1]
	end := len(ix.src)
	if endLine < len(ix.starts) {
		end = ix.starts[endLine]
	}
	return strings.TrimRight(string(ix.src[start:end]), "\n")
}
