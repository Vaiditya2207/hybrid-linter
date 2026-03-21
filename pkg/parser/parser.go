package parser

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
)

// Parser wraps the tree-sitter parser for multiple languages.
type Parser struct {
	engine *sitter.Parser
}

// NewParser creates a new Go parser instance (default).
func NewParser() *Parser {
	return NewParserWithLanguage(golang.GetLanguage())
}

// NewParserForC creates a new C parser instance.
func NewParserForC() *Parser {
	return NewParserWithLanguage(c.GetLanguage())
}

// NewParserForCPP creates a new CPP parser instance.
func NewParserForCPP() *Parser {
	return NewParserWithLanguage(cpp.GetLanguage())
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
