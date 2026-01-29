package parser

import (
	"context"
	"testing"
)

func TestParse(t *testing.T) {
	p := NewParser()
	source := []byte(`
package main
import "fmt"
func main() {
	fmt.Println("hello world")
}
`)
	tree, err := p.Parse(context.Background(), source)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if tree.RootNode() == nil {
		t.Fatal("root node is nil")
	}
	if tree.RootNode().HasError() {
		t.Fatal("root node has error")
	}
}
