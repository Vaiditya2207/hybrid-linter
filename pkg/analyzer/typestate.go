package analyzer

import (
	"strings"
	sitter "github.com/smacker/go-tree-sitter"
)

// ResourcePair defines a set of acquisition and release functions.
type ResourcePair struct {
	Acquire string
	Release string
}

var CommonPairs = []ResourcePair{
	{"mutex_lock", "mutex_unlock"},
	{"spin_lock", "spin_unlock"},
	{"raw_spin_lock", "raw_spin_unlock"},
	{"read_lock", "read_unlock"},
	{"write_lock", "write_unlock"},
}

// ScanForLeaks analyzes a function definition for unbalanced resource pairs.
func ScanForLeaks(root *sitter.Node, source []byte) []Vulnerability {
	var vulns []Vulnerability

	// Walk through all function definitions in the file
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "function_definition" {
			for _, pair := range CommonPairs {
				if leaked := checkPairLeak(n, pair, source); len(leaked) > 0 {
					vulns = append(vulns, leaked...)
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	return vulns
}

func checkPairLeak(funcNode *sitter.Node, pair ResourcePair, source []byte) []Vulnerability {
	// Find all calls to Acquire
	acquires := findCalls(funcNode, pair.Acquire, source)
	if len(acquires) == 0 {
		return nil
	}

	var vulns []Vulnerability
	returns := findReturns(funcNode)
	
	for _, ret := range returns {
		isReleased := false
		
		allReleases := findCalls(funcNode, pair.Release, source)
		
		for _, rel := range allReleases {
			if isNodeBefore(rel, ret) || isDirectlyAfterGoto(funcNode, ret, rel, source) {
				isReleased = true
				break
			}
		}

		if !isReleased {
			content := ret.Content(source)
			if stringsContainsAny(content, "-", "NULL", "ERR_PTR") {
				vulns = append(vulns, Vulnerability{
					ID:        "resource_leak",
					Type:      "ResourceLeak",
					Message:   "Potential " + pair.Acquire + " leak on error return path",
					StartLine: ret.StartPoint().Row + 1,
					StartCol:  ret.StartPoint().Column + 1,
					EndLine:   ret.EndPoint().Row + 1,
					EndCol:    ret.EndPoint().Column + 1,
					FocusNode: ret,
				})
			}
		}
	}

	return vulns
}

func findCalls(n *sitter.Node, funcName string, source []byte) []*sitter.Node {
	var calls []*sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "call_expression" {
			fn := node.ChildByFieldName("function")
			if fn != nil && fn.Content(source) == funcName {
				calls = append(calls, node)
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return calls
}

func findReturns(n *sitter.Node) []*sitter.Node {
	var returns []*sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "return_statement" {
			returns = append(returns, node)
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return returns
}

func isNodeBefore(a, b *sitter.Node) bool {
	return a.EndPoint().Row < b.StartPoint().Row || (a.EndPoint().Row == b.StartPoint().Row && a.EndPoint().Column <= b.StartPoint().Column)
}

func isDirectlyAfterGoto(funcNode, retNode, relNode *sitter.Node, source []byte) bool {
	if relNode.Parent() == retNode.Parent() && isNodeBefore(relNode, retNode) {
		return true
	}
	return false
}

func stringsContainsAny(s string, items ...string) bool {
	for _, item := range items {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}
