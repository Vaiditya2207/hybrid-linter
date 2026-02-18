package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/Vaiditya2207/hybrid-linter/pkg/analyzer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/engine"
	"github.com/Vaiditya2207/hybrid-linter/pkg/slicer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/validator"
)

// Orchestrator coordinates the analysis, context slicing, inference, and validation.
type Orchestrator struct {
	analyzer  *analyzer.Analyzer
	slicer    *slicer.Slicer
	engine    *engine.Engine
	validator *validator.Validator
	maxRetries int
}

// NewOrchestrator creates a new orchestrator pipeline.
func NewOrchestrator(a *analyzer.Analyzer, s *slicer.Slicer, e *engine.Engine, v *validator.Validator, maxRetries int) *Orchestrator {
	return &Orchestrator{
		analyzer:   a,
		slicer:     s,
		engine:     e,
		validator:  v,
		maxRetries: maxRetries,
	}
}

// RepairVulnerability attempts to generate a syntactically valid fix for a vulnerability.
func (o *Orchestrator) RepairVulnerability(ctx context.Context, source []byte, vuln *analyzer.Vulnerability) (string, error) {
	// 1. AST Slicing (Context Extraction)
	siu, err := o.slicer.ExtractContext(source, vuln.FocusNode)
	if err != nil {
		return "", fmt.Errorf("failed to extract context: %w", err)
	}

	for attempt := 1; attempt <= o.maxRetries; attempt++ {
		// 2. Prompt Construction
		prompt := o.buildPrompt(siu, vuln)

		// 3. Inference
		log.Printf("Attempt %d/%d: Generating fix for %s at line %d...", attempt, o.maxRetries, vuln.Type, vuln.StartLine)
		rawOutput, err := o.engine.Predict(ctx, prompt, 256)
		if err != nil {
			return "", fmt.Errorf("inference failed: %w", err)
		}

		// 4. Clean Output
		cleaned := o.validator.CleanOutput(rawOutput)

		// 5. Validation (Parse-Check)
		// To properly validate, we should ideally inject the cleaned snippet back into the SIU context,
		// but for now we'll do a basic syntax check on the isolated snippet. 
		// If the snippet represents a block or a full function, isolated validation works best.
		// If it's just an expression, we might need a dummy wrapper.
		
		// For MVP, we validate the snippet isolated. In the future, we replace the focus node text in the full source.
		valid, _ := o.validator.Validate(ctx, []byte(cleaned))
		
		if valid {
			log.Printf("Attempt %d successful: Valid syntax generated.", attempt)
			return cleaned, nil
		}

		log.Printf("Attempt %d failed: Generated code contains syntax errors. Retrying...", attempt)
	}

	return "", fmt.Errorf("failed to generate a syntactically valid fix after %d attempts", o.maxRetries)
}

func (o *Orchestrator) buildPrompt(siu *slicer.SIU, vuln *analyzer.Vulnerability) string {
	// A strictly formatted prompt mimicking Qwen/Llama instruct formats is required.
	// Since we're using a generic prompt, we'll prefix with basic roles.
	
	prompt := "<|im_start|>system\nYou are an expert Go programmer. Fix the specific vulnerability described. Output ONLY the corrected Go code, with NO markdown formatting, NO explanations. It must be syntactically valid.\n<|im_end|>\n"
	prompt += "<|im_start|>user\n"
	prompt += fmt.Sprintf("Vulnerability Type: %s\n\n", vuln.Type)
	
	if len(siu.Imports) > 0 {
		prompt += fmt.Sprintf("Context Imports:\n%s\n\n", siu.Imports)
	}
	if len(siu.StructDep) > 0 {
		prompt += fmt.Sprintf("Context Types:\n%s\n\n", siu.StructDep)
	}
	
	prompt += fmt.Sprintf("Vulnerable Code to Fix:\n%s\n", siu.FocusContext)
	prompt += "<|im_end|>\n<|im_start|>assistant\n"
	
	return prompt
}
