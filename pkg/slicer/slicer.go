package slicer

import (
	"bytes"

	sitter "github.com/smacker/go-tree-sitter"
)

// SIU represents a Syntax Independent Unit for model context.
type SIU struct {
	Source       []byte
	FocusContext []byte
	Imports      []byte
	StructDep    []byte
	Signatures   []byte
}

// String returns the concatenated context as a string.
func (s *SIU) String() string {
	var buf bytes.Buffer
	if len(s.Imports) > 0 {
		buf.Write(s.Imports)
		buf.WriteString("\n\n")
	}
	if len(s.StructDep) > 0 {
		buf.Write(s.StructDep)
		buf.WriteString("\n\n")
	}
	if len(s.Signatures) > 0 {
		buf.Write(s.Signatures)
		buf.WriteString("\n\n")
	}
	buf.Write(s.FocusContext)
	return buf.String()
}

// Slicer handles AST-based context extraction.
type Slicer struct {
	MaxTokens int
}

// NewSlicer creates a new slicer with the given token limit.
func NewSlicer(maxTokens int) *Slicer {
	return &Slicer{MaxTokens: maxTokens}
}

// ExtractContext builds an SIU from a focus node.
func (s *Slicer) ExtractContext(source []byte, focus *sitter.Node) (*SIU, error) {
	siu := &SIU{Source: source}

	// 1. Focus Context
	siu.FocusContext = source[focus.StartByte():focus.EndByte()]

	// 2. Upward traversal for Struct Definition
	structNode := s.findContainingStructNode(source, focus)
	if structNode != nil {
		siu.StructDep = source[structNode.StartByte():structNode.EndByte()]
	}

	// 3. Root traversal for Imports
	// We need to find the root. A simple way is to traverse up until parent is nil.
	root := focus
	for root.Parent() != nil {
		root = root.Parent()
	}
	siu.Imports = s.extractImports(source, root)

	// 4. Surrounding Signatures
	siu.Signatures = s.extractSurroundingSignatures(source, focus)

	// 5. Pruning to meet token limit
	s.Prune(siu)

	return siu, nil
}

func (s *Slicer) extractSurroundingSignatures(source []byte, node *sitter.Node) []byte {
	// Find if we are in a method or struct
	parentStruct := s.findContainingStructNode(source, node)
	if parentStruct == nil {
		return nil
	}

	// For each field/method in the struct, extract only the name/signature
	// This is simple for now: find all sibling methods or members
	// Actually, easier to search the file for methods with the same receiver type.
	return nil // To be implemented with deeper search
}

func (s *Slicer) findContainingStructNode(source []byte, node *sitter.Node) *sitter.Node {
	curr := node
	var root *sitter.Node
	// Find root first
	temp := node
	for temp.Parent() != nil {
		temp = temp.Parent()
	}
	root = temp

	for curr != nil {
		if curr.Type() == "method_declaration" {
			receiver := curr.ChildByFieldName("receiver")
			if receiver != nil {
				typeName := s.extractReceiverTypeName(source, receiver)
				if typeName != "" {
					return s.findTypeDeclaration(source, root, typeName)
				}
			}
		}
		if curr.Type() == "type_declaration" {
			for i := 0; i < int(curr.ChildCount()); i++ {
				if curr.Child(i).Type() == "struct_type" {
					return curr
				}
			}
		}
		curr = curr.Parent()
	}
	return nil
}

func (s *Slicer) extractReceiverTypeName(source []byte, receiver *sitter.Node) string {
	for i := 0; i < int(receiver.NamedChildCount()); i++ {
		child := receiver.NamedChild(i)
		if child.Type() == "parameter_declaration" {
			typeNode := child.ChildByFieldName("type")
			if typeNode != nil {
				return s.extractIdentifier(source, typeNode)
			}
		}
	}
	return ""
}

func (s *Slicer) extractIdentifier(source []byte, node *sitter.Node) string {
	if node.Type() == "type_identifier" {
		return string(source[node.StartByte():node.EndByte()])
	}
	if node.Type() == "pointer_type" {
		// pointer_type usually has a child that is the base type
		if node.NamedChildCount() > 0 {
			return s.extractIdentifier(source, node.NamedChild(0))
		}
	}
	return ""
}

func (s *Slicer) findTypeDeclaration(source []byte, root *sitter.Node, name string) *sitter.Node {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "type_declaration" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				spec := child.NamedChild(j)
				if spec.Type() == "type_spec" {
					nameNode := spec.ChildByFieldName("name")
					if nameNode != nil {
						typeName := string(source[nameNode.StartByte():nameNode.EndByte()])
						if typeName == name {
							return child
						}
					}
				}
			}
		}
	}
	return nil
}

func (s *Slicer) extractImports(source []byte, root *sitter.Node) []byte {
	var imports [][]byte
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Type() == "import_declaration" {
			imports = append(imports, source[child.StartByte():child.EndByte()])
		}
	}
	return bytes.Join(imports, []byte("\n"))
}

// Prune reduces the size of the SIU to stay within the token limit.
func (s *Slicer) Prune(siu *SIU) {
	for s.EstimateTokens(siu.String()) > s.MaxTokens {
		if len(siu.Signatures) > 0 {
			siu.Signatures = nil
			continue
		}
		if len(siu.StructDep) > 0 {
			siu.StructDep = nil
			continue
		}
		if len(siu.Imports) > 0 {
			siu.Imports = nil
			continue
		}
		break
	}
}

// EstimateTokens provides a rough estimate of token count (1 token ≈ 4 bytes).
func (s *Slicer) EstimateTokens(text string) int {
	return len(text) / 3
}
