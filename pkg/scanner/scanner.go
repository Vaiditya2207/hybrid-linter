package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnoreDirs contains universal directory names that should ALWAYS be aggressively skipped.
// These are typical build artifacts, package managers, and version control directories.
var DefaultIgnoreDirs = map[string]bool{
	"node_modules": true,
	"venv":         true,
	".venv":        true,
	".git":         true,
	".build":       true,
	".swiftpm":     true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
}

// Scanner handles traversing directories and filtering out ignored paths.
type Scanner struct {
	// Extensions is a list of file extensions the scanner should emit (e.g., ".go", ".ts").
	// If empty, it emits all non-ignored files.
	Extensions []string
}

// NewScanner creates a new universal scanner.
func NewScanner(exts []string) *Scanner {
	return &Scanner{Extensions: exts}
}

// FileResult holds the path and content of a discovered file.
type FileResult struct {
	Path    string
	Content []byte
}

// ScanDirectory walks the given directory asynchronously and emits discovered files to the out channel.
// It aggressively skips known vendored/build directories to save I/O and time.
func (s *Scanner) ScanDirectory(ctx context.Context, dir string, out chan<- FileResult) error {
	defer close(out)

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip paths with access errors
		}

		if d.IsDir() {
			// Aggressively skip standard ignored directories
			if DefaultIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a file we care about
		if len(s.Extensions) > 0 {
			matched := false
			for _, ext := range s.Extensions {
				if strings.HasSuffix(d.Name(), ext) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Read the file and push it
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			content, err := os.ReadFile(path)
			if err == nil {
				out <- FileResult{Path: path, Content: content}
			}
		}

		return nil
	})
}
