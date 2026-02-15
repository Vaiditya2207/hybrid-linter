package engine

import (
	"context"
	"os"
	"testing"
)

func TestNewEngineInvalidPath(t *testing.T) {
	_, err := NewEngine("/non/existent/path.gguf")
	if err == nil {
		t.Error("expected error for non-existent model path, got nil")
	}
}

func TestPredictMock(t *testing.T) {
	// Since we don't have a model file in the CI environment, 
	// we skip the actual inference test unless a model is provided.
	modelPath := os.Getenv("HYBRID_LINTER_MODEL_PATH")
	if modelPath == "" {
		t.Skip("Skipping inference test: HYBRID_LINTER_MODEL_PATH not set")
	}

	engine, err := NewEngine(modelPath)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	output, err := engine.Predict(ctx, "Hello", 10)
	if err != nil {
		t.Fatalf("prediction failed: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
	t.Logf("Output: %s", output)
}
