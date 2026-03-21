package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
)

// BuildTypeMap scans the AST for function definitions and identifies those with a 'void' return type.
// It recursively scans local includes if a directory context is provided.
func BuildTypeMap(root *sitter.Node, source []byte, currentDir string, depth int) map[string]bool {
	voidFuncs := make(map[string]bool)
	
	if depth > 3 { // Prevent infinite recursion or too deep scans
		return voidFuncs
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		// 1. Function Definitions: void my_func() { ... }
		if n.Type() == "function_definition" {
			typeNode := n.ChildByFieldName("type")
			if typeNode == nil && n.NamedChildCount() > 0 {
				first := n.NamedChild(0)
				if first.Type() == "primitive_type" || first.Type() == "type_identifier" {
					typeNode = first
				}
			}

			if typeNode != nil {
				typeName := string(source[typeNode.StartByte():typeNode.EndByte()])
				if typeName == "void" {
					declNode := n.ChildByFieldName("declarator")
					if declNode != nil {
						funcName := extractFunctionName(declNode, source)
						if funcName != "" {
							voidFuncs[funcName] = true
						}
					}
				}
			}
		}

		// 2. Function Declarations (Prototypes): void my_func(int x);
		if n.Type() == "declaration" {
			typeNode := n.ChildByFieldName("type")
			if typeNode != nil {
				typeName := string(source[typeNode.StartByte():typeNode.EndByte()])
				if typeName == "void" {
					// In a declaration, there might be multiple declarators, but usually one.
					// We look for function_declarator children.
					for i := 0; i < int(n.ChildCount()); i++ {
						child := n.Child(i)
						if child.Type() == "function_declarator" {
							funcName := extractFunctionName(child, source)
							if funcName != "" {
								voidFuncs[funcName] = true
							}
						}
					}
				}
			}
		}

		// 3. Local Includes: #include "my_header.h"
		if currentDir != "" && n.Type() == "preproc_include" {
			pathNode := n.ChildByFieldName("path")
			if pathNode != nil {
				pathStr := string(source[pathNode.StartByte():pathNode.EndByte()])
				// Only follow local includes (quoted), not system headers (<...>)
				if strings.HasPrefix(pathStr, "\"") && strings.HasSuffix(pathStr, "\"") {
					headerName := strings.Trim(pathStr, "\"")
					headerPath := filepath.Join(currentDir, headerName)
					
					// Follow the include
					headerVoidFuncs := scanHeaderFile(headerPath, depth+1)
					for k, v := range headerVoidFuncs {
						voidFuncs[k] = v
					}
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return voidFuncs
}

func scanHeaderFile(path string, depth int) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	p := sitter.NewParser()
	p.SetLanguage(c.GetLanguage())
	tree, err := p.ParseCtx(context.Background(), nil, data)
	if err != nil {
		return nil
	}

	return BuildTypeMap(tree.RootNode(), data, filepath.Dir(path), depth)
}

// extractFunctionName recursively looks for the identifier in a declarator (handling pointers, etc.)
func extractFunctionName(n *sitter.Node, source []byte) string {
	if n.Type() == "identifier" || n.Type() == "field_identifier" {
		return string(source[n.StartByte():n.EndByte()])
	}
	
	if n.Type() == "function_declarator" {
		// The 'declarator' child of a function_declarator is the name (or another declarator)
		child := n.ChildByFieldName("declarator")
		if child != nil {
			return extractFunctionName(child, source)
		}
	}
	
	if n.Type() == "pointer_declarator" || n.Type() == "parenthesized_declarator" {
		if n.NamedChildCount() > 0 {
			return extractFunctionName(n.NamedChild(0), source)
		}
	}

	// Fallback: check all children
	for i := 0; i < int(n.NamedChildCount()); i++ {
		name := extractFunctionName(n.NamedChild(i), source)
		if name != "" {
			return name
		}
	}

	return ""
}
