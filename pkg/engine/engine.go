package engine

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dianlight/gollama.cpp"
)

// Engine handles communication with the local SLM via gollama.cpp (purego).
type Engine struct {
	ModelPath string
	model     gollama.LlamaModel
	ctx       gollama.LlamaContext
}

// InitBackend initializes the global llama.cpp backend. Must be called on main thread.
func InitBackend() error {
	// Pinning to b6076 because newer versions use .tar.gz which breaks gollama.cpp downloader
	if err := gollama.LoadLibraryWithVersion("b6076"); err != nil {
		return fmt.Errorf("failed to load gollama library: %w", err)
	}
	if err := gollama.Backend_init(); err != nil {
		return fmt.Errorf("failed to initialize backend: %w", err)
	}
	return nil
}

// NewEngine initializes the inference engine model with the given model path.
// This should be called from the thread that will perform inference (e.g., Metal requires this).
func NewEngine(modelPath string) (*Engine, error) {
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("model file does not exist at %s", modelPath)
	}

	// Load model with default params + GPU layers for M1
	mparams := gollama.Model_default_params()
	mparams.NGpuLayers = 32 // Offload to Metal

	model, err := gollama.Model_load_from_file(modelPath, mparams)
	if err != nil {
		return nil, fmt.Errorf("failed to load model: %w", err)
	}

	// Create context. Setting NCtx and NBatch explicitly.
	cparams := gollama.Context_default_params()
	cparams.NCtx = 2048
	cparams.NBatch = 2048
	cparams.NThreads = int32(runtime.NumCPU())

	ctx, err := gollama.Init_from_model(model, cparams)
	if err != nil {
		gollama.Model_free(model)
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	return &Engine{
		ModelPath: modelPath,
		model:     model,
		ctx:       ctx,
	}, nil
}

// Predict sends a prompt to the model and returns the generated repair suggestion.
func (e *Engine) Predict(ctx context.Context, prompt string, maxTokens int) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tokens, err := gollama.Tokenize(e.model, prompt, true, false)
	if err != nil {
		return "", fmt.Errorf("tokenization failed: %w", err)
	}

	// Prepare batch
	batch := gollama.Batch_get_one(tokens)
	// Note: gollama.Batch_get_one batches don't need Batch_free in this version

	if err := gollama.Decode(e.ctx, batch); err != nil {
		return "", fmt.Errorf("initial decode failed: %w", err)
	}

	var output strings.Builder
	sampler := gollama.Sampler_init_greedy()
	// No direct Sampler_free in this version based on README/Source

	for i := 0; i < maxTokens; i++ {
		// Sample next token from the last position (-1)
		token := gollama.Sampler_sample(sampler, e.ctx, -1)
		
		// Check for EOS
		// llama.h defines EOS token. gollama provides Vocab_eos or we can check via vocab
		// For now, let's assume LLAMA_TOKEN_NULL is a stop signal or we add a robust check
		if token == -1 {
			break
		}

		piece := gollama.Token_to_piece(e.model, token, false)
		output.WriteString(piece)

		// Next step: Decode the single new token
		nextTokens := []gollama.LlamaToken{token}
		nextBatch := gollama.Batch_get_one(nextTokens)
		if err := gollama.Decode(e.ctx, nextBatch); err != nil {
			break
		}
	}

	return output.String(), nil
}

// ResetKV flushes the context bounds completely, rescuing bounded Apple Metal RAM environments from slot panics.
func (e *Engine) ResetKV() error {
	if e.ctx != 0 {
		gollama.Free(e.ctx)
	}
	
	cparams := gollama.Context_default_params()
	cparams.NCtx = 2048
	cparams.NBatch = 2048
	cparams.NThreads = int32(runtime.NumCPU())

	ctx, err := gollama.Init_from_model(e.model, cparams)
	if err != nil {
		return fmt.Errorf("failed to reset context: %w", err)
	}
	e.ctx = ctx
	return nil
}

// Close releases the model resources.
func (e *Engine) Close() {
	if e.ctx != 0 {
		gollama.Free(e.ctx)
	}
	if e.model != 0 {
		gollama.Model_free(e.model)
	}
	gollama.Backend_free()
}
