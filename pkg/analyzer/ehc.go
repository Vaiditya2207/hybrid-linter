package analyzer

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// EHCOmission represents a missing cleanup operation identified by Hector.
type EHCReport struct {
	Label     string
	Missing   string
	Reference string // The label that DID have the cleanup
}

// ScanForEHCInconsistencies analyzes functions for inconsistent cleanup patterns across labels.
func ScanForEHCInconsistencies(root *sitter.Node, source []byte) []Vulnerability {
	var vulns []Vulnerability

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n.Type() == "function_definition" {
			if reports := analyzeFunctionEHC(n, source); len(reports) > 0 {
				vulns = append(vulns, reports...)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	return vulns
}

func analyzeFunctionEHC(funcNode *sitter.Node, source []byte) []Vulnerability {
	// 1. Find all labeled statements in the function
	labels := findLabels(funcNode, source)
	if len(labels) < 2 {
		return nil // Need at least two labels to compare
	}

	// 2. Extract the "release stack" for each label
	stacks := make(map[string][]string)
	for labelName, labelNode := range labels {
		visited := make(map[string]bool)
		visited[labelName] = true
		stacks[labelName] = extractCleanupStack(labelNode, labels, source, visited)
	}

	// 3. Compare sibling labels
	// Hector Rule: If multiple error paths lead to different labels,
	// those labels should generally perform the same 'cleanup' for resources
	// that were live at the point of origin.
	// Microscopic Heuristic: If Label A releases {X, Y, Z} and Label B releases {X, Y},
	// then Z is likely missing from Label B.
	var vulns []Vulnerability
	for nameA, stackA := range stacks {
		for nameB, stackB := range stacks {
			if nameA == nameB {
				continue
			}

			// If stackA is a strict superset of stackB
			if isStrictSuperset(stackA, stackB) {
				diff := getDifference(stackA, stackB)
				for _, missing := range diff {
					// We only care about "release-like" functions
					if !isReleaseLike(missing) {
						continue
					}

					labelNode := labels[nameB]
					vulns = append(vulns, Vulnerability{
						ID:        "ehc_omission",
						Type:      "EHCOmission",
						Message:   "Inconsistent cleanup: label '" + nameB + "' misses '" + missing + "' (present in '" + nameA + "')",
						StartLine: labelNode.StartPoint().Row + 1,
						StartCol:  labelNode.StartPoint().Column + 1,
						EndLine:   labelNode.EndPoint().Row + 1,
						EndCol:    labelNode.EndPoint().Column + 1,
						FocusNode: labelNode,
					})
				}
			}
		}
	}

	return deduplicateVulns(vulns)
}

func findLabels(n *sitter.Node, source []byte) map[string]*sitter.Node {
	labels := make(map[string]*sitter.Node)
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "labeled_statement" {
			if node.ChildCount() > 0 {
				name := node.Child(0).Content(source)
				labels[name] = node
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return labels
}

func extractCleanupStack(n *sitter.Node, labels map[string]*sitter.Node, source []byte, visited map[string]bool) []string {
	var stack []string
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == "call_expression" {
			fn := node.ChildByFieldName("function")
			if fn != nil {
				stack = append(stack, fn.Content(source))
			}
		}
		// Follow 'goto' to other labels (chained cleanup)
		if node.Type() == "goto_statement" {
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "statement_identifier" || child.Type() == "identifier" {
					targetName := child.Content(source)
					if targetNode, ok := labels[targetName]; ok && !visited[targetName] {
						visited[targetName] = true
						// Recursively add target label's cleanup stack
						subStack := extractCleanupStack(targetNode, labels, source, visited)
						stack = append(stack, subStack...)
					}
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return stack
}

func isStrictSuperset(a, b []string) bool {
	if len(a) <= len(b) {
		return false
	}
	// Every element in b must be in a
	mapA := make(map[string]bool)
	for _, s := range a {
		mapA[s] = true
	}
	for _, s := range b {
		if !mapA[s] {
			return false
		}
	}
	return true
}

func getDifference(a, b []string) []string {
	var diff []string
	mapB := make(map[string]bool)
	for _, s := range b {
		mapB[s] = true
	}
	for _, s := range a {
		if !mapB[s] {
			diff = append(diff, s)
		}
	}
	return diff
}

func isReleaseLike(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "unlock") || 
		strings.Contains(lower, "free") || 
		strings.Contains(lower, "put") || 
		strings.Contains(lower, "unregister") ||
		strings.Contains(lower, "release") ||
		strings.Contains(lower, "destroy")
}

func deduplicateVulns(vulns []Vulnerability) []Vulnerability {
	unique := make(map[string]Vulnerability)
	for _, v := range vulns {
		key := fmt.Sprintf("%s_%d", v.Message, v.StartLine)
		unique[key] = v
	}
	var res []Vulnerability
	for _, v := range unique {
		res = append(res, v)
	}
	return res
}
