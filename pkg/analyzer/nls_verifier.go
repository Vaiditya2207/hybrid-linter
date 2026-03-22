package analyzer

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// VerifyNLSConstraints checks a function body against its extracted semantic documentation.
func (a *Analyzer) VerifyNLSConstraints(funcNode *sitter.Node, source []byte, constraint *SemanticConstraint) []Vulnerability {
	var vulns []Vulnerability

	if constraint == nil {
		return nil
	}

	// 1. Verify Return Guards (Parameter validation)
	// Example: Doc says "returns -EINVAL if foo is NULL"
	for _, guard := range constraint.ReturnGuards {
		if !findReturnGuardInBody(funcNode, guard, source) {
			vulns = append(vulns, Vulnerability{
				ID:        "semantic_divergence",
				Type:      "SemanticDivergence",
				Message:   fmt.Sprintf("Documentation-Code Divergence: Function claims to return %s if %s, but no such guard found.", guard.Value, guard.Condition),
				StartLine: funcNode.StartPoint().Row + 1,
				StartCol:  funcNode.StartPoint().Column + 1,
				EndLine:   funcNode.EndPoint().Row + 1,
				EndCol:    funcNode.EndPoint().Column + 1,
				FocusNode: funcNode,
			})
		}
	}

	// 2. Verify Lock Requirements
	// Example: Doc says "Note: Caller must hold tree_lock"
	for _, lock := range constraint.LockRequires {
		if !isLockVerifiedInBody(funcNode, lock, source) {
			vulns = append(vulns, Vulnerability{
				ID:        "semantic_divergence",
				Type:      "SemanticDivergence",
				Message:   fmt.Sprintf("Locking Divergence: Function requires lock '%s' held by caller, but misses lockdep_assert_held() or similar verification.", lock),
				StartLine: funcNode.StartPoint().Row + 1,
				StartCol:  funcNode.StartPoint().Column + 1,
				EndLine:   funcNode.EndPoint().Row + 1,
				EndCol:    funcNode.EndPoint().Column + 1,
				FocusNode: funcNode,
			})
		}
	}

	return vulns
}

func findReturnGuardInBody(funcNode *sitter.Node, guard ReturnGuard, source []byte) bool {
	// Structural heuristic: look for an if statement that contains the return value
	found := false
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if found {
			return
		}
		if n.Type() == "if_statement" {
			content := n.Content(source)
			if strings.Contains(content, guard.Value) {
				// Condition check is harder statically, but if the return value matches, 
				// we assume the guard is present for now.
				found = true
				return
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(funcNode)
	return found
}

func isLockVerifiedInBody(funcNode *sitter.Node, lock string, source []byte) bool {
	// Look for lockdep_assert_held(lock) or similar
	found := false
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if found {
			return
		}
		if n.Type() == "call_expression" {
			content := n.Content(source)
			if strings.Contains(content, "lockdep_assert_held") && strings.Contains(content, lock) {
				found = true
				return
			}
			// In some kernels, they use assertions or macros
			if (strings.Contains(content, "assert") || strings.Contains(content, "ASSERT")) && strings.Contains(content, lock) {
				found = true
				return
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(funcNode)
	return found
}
