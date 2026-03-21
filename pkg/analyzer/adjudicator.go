package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/Vaiditya2207/hybrid-linter/pkg/engine"
	"github.com/Vaiditya2207/hybrid-linter/pkg/slicer"
)

// Adjudicator uses an LLM to prune false positive vulnerabilities.
type Adjudicator struct {
	engine *engine.Engine
	slicer *slicer.Slicer
}

// NewAdjudicator creates a new neural judge.
func NewAdjudicator(eng *engine.Engine) *Adjudicator {
	return &Adjudicator{
		engine: eng,
		slicer: slicer.NewSlicer(1024),
	}
}

// Judge evaluates if a vulnerability path is semantically reachable.
func (a *Adjudicator) Judge(ctx context.Context, source []byte, vuln *Vulnerability) (bool, string) {
	if a.engine == nil {
		return true, "No engine available"
	}

	// Extract context around the vulnerability
	contextSIU, err := a.slicer.ExtractContext(source, vuln.FocusNode)
	if err != nil {
		return true, fmt.Sprintf("Slicing error: %v", err)
	}

	prompt := fmt.Sprintf(`### Task: Program Analysis Adjudication
Analyze the C/Go code below and determine if the reported issue is a genuine, reachable vulnerability or a false positive.

### Reported Issue:
Type: %s
Message: %s
Line: %d

### Code Context:
%s

### Decision:
Is this bug path reachable in a real execution? Answer only "YES" or "NO" followed by a 1-sentence reason.`, 
	vuln.Type, vuln.Message, vuln.StartLine, contextSIU.String())

	resp, err := a.engine.Predict(ctx, prompt, 128)
	if err != nil {
		return true, fmt.Sprintf("LLM error: %v", err)
	}

	trimmed := strings.ToUpper(strings.TrimSpace(resp))
	if strings.HasPrefix(trimmed, "NO") {
		return false, resp
	}

	return true, resp
}
