package analyzer

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// ScanForMustCheck analyzes function definitions to see if they return error sentinels.
func ScanForMustCheck(root *sitter.Node, source []byte) map[string]bool {
	mustCheck := make(map[string]bool)

	// Query for function definitions and their return statements
	// We'll use a manual walk since we want to look inside bodies.
	walkFunctions(root, source, mustCheck)

	return mustCheck
}

func walkFunctions(n *sitter.Node, source []byte, mustCheck map[string]bool) {
	if n.Type() == "function_definition" {
		funcName := ""
		// Find declarator -> identifier
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "function_declarator" {
				// C/C++ style
				for j := 0; j < int(child.ChildCount()); j++ {
					grand := child.Child(j)
					if grand.Type() == "identifier" || grand.Type() == "field_identifier" {
						funcName = grand.Content(source)
						break
					}
				}
			}
		}

		if funcName != "" {
			if isErrorReturning(n, source) {
				mustCheck[funcName] = true
			}
		}
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		walkFunctions(n.Child(i), source, mustCheck)
	}
}

func isErrorReturning(n *sitter.Node, source []byte) bool {
	// Walk the body looking for return statements
	returnsError := false
	
	var findReturns func(*sitter.Node)
	findReturns = func(node *sitter.Node) {
		if node.Type() == "return_statement" {
			// Check the expression
			if node.ChildCount() > 1 {
				expr := node.Child(1)
				content := expr.Content(source)
				
				// Heuristics for error sentinels
				if strings.HasPrefix(content, "-") { // -1, -ENOMEM
					returnsError = true
				} else if content == "NULL" || content == "nullptr" {
					returnsError = true
				} else if strings.Contains(content, "ERR_PTR") {
					returnsError = true
				} else if strings.HasPrefix(content, "ERR_") {
					returnsError = true
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			findReturns(node.Child(i))
		}
	}

	findReturns(n)
	return returnsError
}
