package slicer

import (
	"context"
	"testing"

	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
)

func TestExtractContext(t *testing.T) {
	p := parser.NewParser()
	s := NewSlicer(1024)

	source := []byte(`
package main
import "fmt"
type MyStruct struct {
	Field int
}
func (m *MyStruct) MyMethod() {
	fmt.Println("hello")
}
func main() {
	var m MyStruct
	m.MyMethod()
}
`)

	tree, err := p.Parse(context.Background(), source)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Find the MyMethod node
	// For testing, let's just find any node deeply
	root := tree.RootNode()
	methodNode := root.NamedChild(2).NamedChild(0) // This is a bit fragile, let's find it by type
	
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "method_declaration" {
			methodNode = child
			break
		}
	}

	if methodNode == nil || methodNode.Type() != "method_declaration" {
		t.Fatalf("failed to find method_declaration node, found %s", methodNode.Type())
	}

	siu, err := s.ExtractContext(source, methodNode)
	if err != nil {
		t.Fatalf("failed to extract context: %v", err)
	}

	if len(siu.FocusContext) == 0 {
		t.Error("expected focus context, got empty")
	}

	if len(siu.Imports) == 0 {
		t.Error("expected imports, got empty")
	}

	if len(siu.StructDep) == 0 {
		t.Error("expected struct dep, got empty")
	}

	t.Logf("Extracted Context:\n%s", siu.String())
}

func TestPrune(t *testing.T) {
	s := NewSlicer(20) // Very small limit for testing

	siu := &SIU{
		FocusContext: []byte("func main() {}"),
		Imports:      []byte("import \"fmt\"\nimport \"os\""),
		StructDep:    []byte("type T struct { A int }"),
		Signatures:   []byte("func (t *T) M()"),
	}

	initialTokens := s.EstimateTokens(siu.String())
	t.Logf("Initial Tokens: %d", initialTokens)

	s.Prune(siu)

	finalTokens := s.EstimateTokens(siu.String())
	t.Logf("Final Tokens: %d", finalTokens)

	if finalTokens > s.MaxTokens {
		t.Errorf("expected tokens <= %d, got %d", s.MaxTokens, finalTokens)
	}

	if len(siu.Signatures) != 0 {
		t.Error("expected Signatures to be pruned")
	}
}
