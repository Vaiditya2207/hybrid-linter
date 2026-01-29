package analyzer

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// Vulnerability represents a detected issue in the code.
type Vulnerability struct {
	ID        string
	Type      string
	Message   string
	StartLine uint32
	StartCol  uint32
	EndLine   uint32
	EndCol    uint32
	FocusNode *sitter.Node
}

// Analyzer runs SCM queries against a tree.
type Analyzer struct {
	Language *sitter.Language
}

// NewAnalyzer creates a new analyzer for Go.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		Language: golang.GetLanguage(),
	}
}

// Analyze runs the given SCM query against the root node of a tree.
func (a *Analyzer) Analyze(ctx context.Context, root *sitter.Node, queryData []byte) ([]Vulnerability, error) {
	q, err := sitter.NewQuery(queryData, a.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, root)

	var vulnerabilities []Vulnerability
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, cap := range m.Captures {
			captureName := q.CaptureNameForId(cap.Index)
			node := cap.Node

			vulnerabilities = append(vulnerabilities, Vulnerability{
				ID:        captureName,
				Type:      captureName,
				Message:   fmt.Sprintf("Detected %s", captureName),
				StartLine: node.StartPoint().Row + 1,
				StartCol:  node.StartPoint().Column + 1,
				EndLine:   node.EndPoint().Row + 1,
				EndCol:    node.EndPoint().Column + 1,
				FocusNode: node,
			})
		}
	}

	return vulnerabilities, nil
}

// LoadQueryFromFile loads a query from a file path.
func LoadQueryFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
