package analyzer

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// IsVariableHandled checks if a variable assigned at 'assignmentNode' is used in a conditional or passed to a function.
func IsVariableHandled(root *sitter.Node, varName string, startRow uint32, source []byte) bool {
	handled := false

	// Define what "handled" means
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if handled {
			return
		}

		// Only look at nodes after the startRow
		if n.StartPoint().Row < startRow {
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			return
		}

		// 1. Used in an 'if' statement condition
		if n.Type() == "if_statement" {
			condition := n.ChildByFieldName("condition")
			if condition != nil && stringsContainsVar(condition, varName, source) {
				handled = true
				return
			}
		}

		// 2. Passed to a function (e.g., IS_ERR(ret))
		if n.Type() == "call_expression" {
			args := n.ChildByFieldName("arguments")
			if args != nil && stringsContainsVar(args, varName, source) {
				handled = true
				return
			}
		}
		
		// 3. Used in a return statement
		if n.Type() == "return_statement" {
			if stringsContainsVar(n, varName, source) {
				handled = true
				return
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return handled
}

func stringsContainsVar(n *sitter.Node, varName string, source []byte) bool {
	// Precise check for identifier
	contains := false
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if contains {
			return
		}
		if node.Type() == "identifier" || node.Type() == "field_identifier" {
			if node.Content(source) == varName {
				contains = true
				return
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return contains
}
