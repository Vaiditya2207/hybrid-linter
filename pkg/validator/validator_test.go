package validator

import (
	"context"
	"testing"

	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
)

func TestValidator_Validate(t *testing.T) {
	p := parser.NewParser()
	v := NewValidator(p)

	ctx := context.Background()

	tests := []struct {
		name    string
		source  string
		isValid bool
	}{
		{
			name:    "Valid Function",
			source:  "func main() { fmt.Println(\"ok\") }",
			isValid: true,
		},
		{
			name:    "Missing Brace",
			source:  "func main() { fmt.Println(\"err\") ",
			isValid: false,
		},
		{
			name:    "Invalid Syntax",
			source:  "func var 1main() { }",
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := v.Validate(ctx, []byte(tt.source))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if valid != tt.isValid {
				t.Errorf("expected %v, got %v", tt.isValid, valid)
			}
		})
	}
}

func TestValidator_CleanOutput(t *testing.T) {
	v := NewValidator(nil)

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{
			name:     "No formatting",
			raw:      "func main() {}",
			expected: "func main() {}",
		},
		{
			name:     "Markdown Go block",
			raw:      "```go\nfunc main() {}\n```",
			expected: "func main() {}",
		},
		{
			name:     "Markdown generic block",
			raw:      "```\nvar x int = 5\n```",
			expected: "var x int = 5",
		},
		{
			name:     "Whitespace padded",
			raw:      "\n  ```go\nfunc main() {}\n```  \n",
			expected: "func main() {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := v.CleanOutput(tt.raw)
			if cleaned != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, cleaned)
			}
		})
	}
}
