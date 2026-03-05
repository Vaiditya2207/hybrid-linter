#!/bin/bash
set -e

echo "🔄 Resetting test files..."
cat << 'EOF' > test/v3_repairs/micro_bug.go
package main

import (
	"os"
)

// mini should return an error if ReadFile fails
func mini() error {
	_, err := os.ReadFile("nonexistent.txt")
	_ = err
	return nil
}
EOF

cat << 'EOF' > test/v3_repairs/micro_json.go
package main

import "encoding/json"

// parse should return an error if Unmarshal fails
func parse(b []byte) error {
	var m map[string]string
	err := json.Unmarshal(b, &m)
	_ = err
	return nil
}
EOF

echo ""
echo "🧩 Running Baseline Tests (Should FAIL because bugs exist)..."
if go test ./test/v3_repairs -v; then
    echo "❌ Baseline tests PASSED? That means the tests are broken. Exiting."
    exit 1
else
    echo "✅ Baseline tests successfully FAILED."
fi

echo ""
echo "🚀 Running Automated 3B LLM Repair Pipeline..."
# Run the pipeline silently. The 3B model takes slightly longer but has superior logical coherence.
# Note: HYBRID_CHAT is NOT set, so it will autonomously /apply!
go run ./cmd/hybrid-linter -dir ./test/v3_repairs -repair -model ./models/llama-3.2-3b-instruct-q4_k_m.gguf

echo ""
echo "🧪 Running Validation Tests on Patched AST (Should PASS)..."
go test ./test/v3_repairs -v

echo "🎉 Phase 9 E2E Verification Complete!"
