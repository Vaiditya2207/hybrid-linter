package validator

import (
	"context"
	"fmt"
	"strings"

	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// Validator uses Tree-sitter to ensure generated code is syntactically correct.
type Validator struct {
	parser *parser.Parser
}

// NewValidator initializes a new syntax validator.
func NewValidator(p *parser.Parser) *Validator {
	if p == nil {
		p = parser.NewParser()
	}
	return &Validator{parser: p}
}

// Validate takes a snippet of Go code (combined or isolated) and checks for syntax errors.
func (v *Validator) Validate(ctx context.Context, source []byte) (bool, error) {
	tree, err := v.parser.Parse(ctx, source)
	if err != nil {
		return false, fmt.Errorf("failed to parse generated code: %w", err)
	}

	hasError := v.hasSyntaxError(tree.RootNode())
	return !hasError, nil
}

// hasSyntaxError recursively checks for ERROR or MISSING nodes.
func (v *Validator) hasSyntaxError(node *sitter.Node) bool {
	if node.HasError() {
		return true
	}
	// Tree-sitter node.IsMissing() indicates a node was expected but not found
	if node.IsMissing() {
		return true
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		if v.hasSyntaxError(node.Child(i)) {
			return true
		}
	}
	return false
}

// CleanOutput applies heuristics to clean up raw LLM text (e.g., removing markdown fences).
func (v *Validator) CleanOutput(raw string) string {
	cleaned := strings.TrimSpace(raw)
	
	// Remove markdown code blocks if present
	if strings.HasPrefix(cleaned, "```go") {
		cleaned = strings.TrimPrefix(cleaned, "```go")
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
	}
	
	cleaned = strings.TrimSuffix(cleaned, "```")
	
	return strings.TrimSpace(cleaned)
}
