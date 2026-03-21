package analyzer

import (
	"context"
	"testing"

	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
)

func TestAnalyzeUnhandledError(t *testing.T) {
	p := parser.NewParser()
	a := NewAnalyzer()

	// Note: The query might need adjustment based on the exact AST structure.
	// For this test, we'll use a simpler source to match the query.
	sourceWithIssue := []byte(`
package main
func doSomething() error { return nil }
func main() {
	err := doSomething()
}
`)

	tree, err := p.Parse(context.Background(), sourceWithIssue)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	query := `
(
  (short_var_declaration
    left: (expression_list
      (identifier) @err)
    right: (expression_list
      (call_expression) @call))
  (#match? @err "^err$")
)
`
	vulns, err := a.Analyze(context.Background(), tree.RootNode(), sourceWithIssue, []byte(query), nil, nil, nil, "")
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	if len(vulns) == 0 {
		t.Error("expected at least one vulnerability, found none")
	}

	for _, v := range vulns {
		t.Logf("Found vulnerability: %s at %d:%d", v.Type, v.StartLine, v.StartCol)
	}
}
