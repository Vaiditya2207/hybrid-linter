package analyzer

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed rules/*.scm
var EmbeddedRules embed.FS

// LoadEmbeddedQuery loads a built-in query by its baseline filename (e.g., "unhandled_errors").
func LoadEmbeddedQuery(name string) ([]byte, error) {
	path := fmt.Sprintf("rules/%s.scm", name)
	data, err := fs.ReadFile(EmbeddedRules, path)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded rule %s: %w", name, err)
	}
	return data, nil
}
