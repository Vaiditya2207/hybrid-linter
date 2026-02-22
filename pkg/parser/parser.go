package parser

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// Parser wraps the tree-sitter parser for Go.
type Parser struct {
	engine *sitter.Parser
}

// NewParser creates a new Go parser instance (default for V1).
func NewParser() *Parser {
	return NewParserWithLanguage(golang.GetLanguage())
}

// NewParserWithLanguage enables Phase 7 V2 architecture, dynamically
// wrapping new Tree-sitter syntax bindings without hardcoded behavior.
func NewParserWithLanguage(lang *sitter.Language) *Parser {
	p := sitter.NewParser()
	p.SetLanguage(lang)
	return &Parser{engine: p}
}

// Parse parses the given source code and returns the tree.
func (p *Parser) Parse(ctx context.Context, source []byte) (*sitter.Tree, error) {
	return p.engine.ParseCtx(ctx, nil, source)
}

// ParseIncremental performs an incremental parse using the old tree and source.
func (p *Parser) ParseIncremental(ctx context.Context, oldTree *sitter.Tree, source []byte) (*sitter.Tree, error) {
	return p.engine.ParseCtx(ctx, oldTree, source)
}
