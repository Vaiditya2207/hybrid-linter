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

// ScanForLeaks analyzes function definitions for unbalanced resource pairs.
// It uses goto-aware path analysis to understand kernel cleanup patterns.
func ScanForLeaks(root *sitter.Node, source []byte) []Vulnerability {
	var vulns []Vulnerability

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
	acquires := findCallNodes(funcNode, pair.Acquire, source)
	if len(acquires) == 0 {
		return nil
	}

	// Build a map of labels -> their cleanup actions
	labelReleases := buildLabelReleaseMap(funcNode, pair.Release, source)

	// Check: does the function have ANY release call at all?
	allReleases := findCallNodes(funcNode, pair.Release, source)
	if len(allReleases) > 0 {
		// If there are releases in the function, the developer is aware.
		// Only flag if there's a SPECIFIC error return that bypasses ALL release paths.
		return checkSpecificLeakPaths(funcNode, pair, allReleases, labelReleases, source)
	}

	// No releases at all in a function that acquires -- this is a genuine leak.
	// But only flag it once, not per-return.
	acq := acquires[0]
	return []Vulnerability{{
		ID:        "resource_leak",
		Type:      "ResourceLeak",
		Message:   pair.Acquire + " acquired but " + pair.Release + " never called in this function",
		StartLine: acq.StartPoint().Row + 1,
		StartCol:  acq.StartPoint().Column + 1,
		EndLine:   acq.EndPoint().Row + 1,
		EndCol:    acq.EndPoint().Column + 1,
		FocusNode: acq,
	}}
}

// checkSpecificLeakPaths finds error returns that bypass all release paths.
func checkSpecificLeakPaths(funcNode *sitter.Node, pair ResourcePair, releases []*sitter.Node, labelReleases map[string]bool, source []byte) []Vulnerability {
	var vulns []Vulnerability

	// Find all return statements
	returns := findReturnNodes(funcNode)

	for _, ret := range returns {
		// Check if this return has a release BEFORE it in the same block scope
		hasDirectRelease := false
		for _, rel := range releases {
			if isNodeBefore(rel, ret) && sameBlockScope(rel, ret) {
				hasDirectRelease = true
				break
			}
		}

		if hasDirectRelease {
			continue
		}

		// Check if this return is at a label that includes a release
		label := findContainingLabel(funcNode, ret, source)
		if label != "" && labelReleases[label] {
			continue
		}

		// Check if there's a goto BEFORE this return that jumps to a label with a release
		if hasGotoToReleaseLabel(funcNode, ret, labelReleases, source) {
			continue
		}

		// This return genuinely has no release on its path.
		// But only flag it if it looks like an EARLY return (not the final return).
		// The final return in a function is usually the "happy path" or the cleanup exit.
		if isLastReturn(funcNode, ret) {
			continue
		}

		vulns = append(vulns, Vulnerability{
			ID:        "resource_leak",
			Type:      "ResourceLeak",
			Message:   "Potential " + pair.Acquire + " leak: early return without " + pair.Release,
			StartLine: ret.StartPoint().Row + 1,
			StartCol:  ret.StartPoint().Column + 1,
			EndLine:   ret.EndPoint().Row + 1,
			EndCol:    ret.EndPoint().Column + 1,
			FocusNode: ret,
		})
	}

	return vulns
}

// buildLabelReleaseMap scans for labeled_statement nodes and checks if any contain a release call.
func buildLabelReleaseMap(funcNode *sitter.Node, releaseName string, source []byte) map[string]bool {
	labels := make(map[string]bool)
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "labeled_statement" {
			// First child is the label identifier
			if n.ChildCount() > 0 {
				labelNode := n.Child(0)
				labelName := labelNode.Content(source)
				// Check if any descendant of this labeled_statement calls the release function
				calls := findCallNodes(n, releaseName, source)
				if len(calls) > 0 {
					labels[labelName] = true
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(funcNode)
	return labels
}

// hasGotoToReleaseLabel checks if there's a goto statement before the return
// that jumps to a label known to release the resource.
func hasGotoToReleaseLabel(funcNode, retNode *sitter.Node, labelReleases map[string]bool, source []byte) bool {
	found := false
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if found {
			return
		}
		if n.Type() == "goto_statement" && isNodeBefore(n, retNode) {
			// Extract the label name from the goto
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "statement_identifier" || child.Type() == "identifier" {
					label := child.Content(source)
					if labelReleases[label] {
						found = true
						return
					}
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(funcNode)
	return found
}

// findContainingLabel returns the label name if the node is inside a labeled_statement.
func findContainingLabel(funcNode, node *sitter.Node, source []byte) string {
	parent := node.Parent()
	for parent != nil && parent != funcNode {
		if parent.Type() == "labeled_statement" && parent.ChildCount() > 0 {
			return parent.Child(0).Content(source)
		}
		parent = parent.Parent()
	}
	return ""
}

// sameBlockScope checks if two nodes share the same immediate parent compound_statement.
func sameBlockScope(a, b *sitter.Node) bool {
	return a.Parent() == b.Parent()
}

// isLastReturn checks if this is the last return statement in the function.
func isLastReturn(funcNode, ret *sitter.Node) bool {
	allReturns := findReturnNodes(funcNode)
	if len(allReturns) == 0 {
		return false
	}
	last := allReturns[len(allReturns)-1]
	return last.StartPoint().Row == ret.StartPoint().Row && last.StartPoint().Column == ret.StartPoint().Column
}

func findCallNodes(n *sitter.Node, funcName string, source []byte) []*sitter.Node {
	var calls []*sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "call_expression" {
			fn := node.ChildByFieldName("function")
			if fn != nil {
				name := fn.Content(source)
				// Match exact name or variants like spin_lock_irqsave -> spin_lock prefix
				if name == funcName || strings.HasPrefix(name, funcName+"_") {
					calls = append(calls, node)
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return calls
}

func findReturnNodes(n *sitter.Node) []*sitter.Node {
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
	return a.EndPoint().Row < b.StartPoint().Row ||
		(a.EndPoint().Row == b.StartPoint().Row && a.EndPoint().Column <= b.StartPoint().Column)
}
