package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// SemanticConstraint represents an extracted API contract from kernel-doc.
type SemanticConstraint struct {
	FunctionName   string        `json:"function_name"`
	ReturnGuards   []ReturnGuard `json:"return_guards"`
	LockRequires   []string      `json:"lock_requires"`
	ContextType    string        `json:"context_type"` // e.g., "process", "atomic", "irq"
	SleepAllowed   bool          `json:"sleep_allowed"`
	CustomNotes    []string      `json:"custom_notes"`
}

type ReturnGuard struct {
	Condition string `json:"condition"` // e.g., "foo is NULL"
	Value     string `json:"value"`     // e.g., "-EINVAL"
}

// ExtractDocConstraints identifies the comment block for a function and uses an LLM to parse it.
func (a *Adjudicator) ExtractDocConstraints(ctx context.Context, funcNode *sitter.Node, source []byte) (*SemanticConstraint, error) {
	if a.engine == nil {
		return nil, fmt.Errorf("no engine available for NLS extraction")
	}

	// 1. Find the comment block preceding the function
	comment := findPrecedingComment(funcNode, source)
	if comment == "" {
		return nil, nil // No documentation to parse
	}

	// 2. Extract function signature for context
	sig := extractFunctionSignature(funcNode, source)

	// 3. Prompt the LLM to extract structured constraints
	prompt := fmt.Sprintf(`### Task: Natural Language Specification (NLS) Extraction
Analyze the kernel-doc comment and function signature below. 
Extract the formal API contract into a JSON object.

### Target Schema:
{
  "function_name": "string",
  "return_guards": [{"condition": "string", "value": "string"}],
  "lock_requires": ["string"],
  "context_type": "process|atomic|irq",
  "sleep_allowed": bool,
  "custom_notes": ["string"]
}

### Code context:
Signature: %s
Comment:
%s

### Output (JSON only):`, sig, comment)

	resp, err := a.engine.Predict(ctx, prompt, 512)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// 4. Parse the JSON response
	// Clean up markdown code blocks if the LLM included them
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var constraint SemanticConstraint
	if err := json.Unmarshal([]byte(resp), &constraint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NLS JSON: %w", err)
	}

	return &constraint, nil
}

func findPrecedingComment(funcNode *sitter.Node, source []byte) string {
	var comments []string
	curr := funcNode.PrevSibling()
	
	// Walk backwards through siblings to find all adjacent comments
	for curr != nil {
		if curr.Type() == "comment" {
			comments = append([]string{curr.Content(source)}, comments...)
			curr = curr.PrevSibling()
		} else if curr.Type() == " " || curr.Type() == "\n" {
			// Skip whitespace
			curr = curr.PrevSibling()
		} else {
			break
		}
	}
	
	if len(comments) == 0 {
		// Try parent's previous sibling (sometimes comments are outside the declaration scope)
		if funcNode.Parent() != nil {
			return findPrecedingComment(funcNode.Parent(), source)
		}
		return ""
	}

	return strings.Join(comments, "\n")
}

func extractFunctionSignature(funcNode *sitter.Node, source []byte) string {
	// Root of a function_definition usually contains the result type, declarator, and parameters
	// We just want the top part before the compound_statement (body)
	for i := 0; i < int(funcNode.ChildCount()); i++ {
		child := funcNode.Child(i)
		if child.Type() == "compound_statement" {
			// Stop before the body
			return funcNode.Content(source)[:child.StartByte()-funcNode.StartByte()]
		}
	}
	return funcNode.Content(source)
}
