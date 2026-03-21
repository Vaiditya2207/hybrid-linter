package health

import (
	"context"
	"strings"
	"time"

	"github.com/Vaiditya2207/hybrid-linter/pkg/analyzer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
	"github.com/Vaiditya2207/hybrid-linter/pkg/scanner"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// HealthIssue represents a single templated diagnostic.
type HealthIssue struct {
	File        string
	Line        int
	CodeSnippet string
	Type        string
	Severity    string
	Solution    string
}

// CodebaseHealth holds aggregated metadata about the project's structural integrity.
type CodebaseHealth struct {
	TotalFiles       int
	TotalLines       int
	Vulnerabilities  int
	ComplexityScore  int
	FileDistribution map[string]int
	AnalysisTime     time.Duration
	Issues           []HealthIssue
}

// Scorer analyzes a set of files to produce a health report.
type Scorer struct {
	analyzer  *analyzer.Analyzer
	queryData []byte
}

func NewScorer(a *analyzer.Analyzer, qd []byte) *Scorer {
	return &Scorer{analyzer: a, queryData: qd}
}

// GenerateScore walks a directory using the Phase 7 scanner and computes metrics.
func (s *Scorer) GenerateScore(ctx context.Context, dir string) (*CodebaseHealth, error) {
	start := time.Now()
	health := &CodebaseHealth{
		FileDistribution: make(map[string]int),
	}

	// Use our new Phase 7 Scanner to skip node_modules/venv etc.
	scan := scanner.NewScanner([]string{".go", ".c", ".py", ".ts", ".js", ".swift", ".zig"})
	fileChan := make(chan scanner.FileResult, 100)

	go func() {
		scan.ScanDirectory(ctx, dir, fileChan)
	}()

	// Temporary V3 MVP: Hardcoded to Golang grammar for the health check.
	// In final V3 this will use the lsp.Registry.
	p := parser.NewParserWithLanguage(golang.GetLanguage())

	for file := range fileChan {
		health.TotalFiles++
		// simple extension tracker
		ext := ""
		if idx := strings.LastIndex(file.Path, "."); idx != -1 {
			ext = file.Path[idx+1:]
		}
		health.FileDistribution[ext]++

		lines := strings.Count(string(file.Content), "\n")
		health.TotalLines += lines

		tree, err := p.Parse(ctx, file.Content)
		if err != nil {
			continue
		}

		// Count unhandled errors as a primary health metric
		// Using the embedded rules from Phase 6
		vulns, err := s.analyzer.Analyze(ctx, tree.RootNode(), s.queryData)
		if err == nil {
			health.Vulnerabilities += len(vulns)
			for _, v := range vulns {
				snip := string(file.Content[v.FocusNode.StartByte():v.FocusNode.EndByte()])
				snip = strings.ReplaceAll(snip, "\n", " ")
				health.Issues = append(health.Issues, HealthIssue{
					File:        file.Path,
					Line:        int(v.StartLine),
					CodeSnippet: snip,
					Type:        "Unhandled Error Definition",
					Severity:    "High",
					Solution:    "Implement explicit error checking (e.g., `if err != nil { return err }`) immediately after the variable declaration to prevent silent propagation.",
				})
			}
		}

		// Heuristic Complexity: Count functions and branches
		health.ComplexityScore += s.estimateComplexity(tree.RootNode())
	}

	health.AnalysisTime = time.Since(start)
	return health, nil
}

func (s *Scorer) estimateComplexity(n *sitter.Node) int {
	count := 0
	// Basic complexity nodes in most grammars (if, for, func)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		child := n.NamedChild(i)
		t := child.Type()
		if strings.Contains(t, "if_statement") || strings.Contains(t, "for_statement") || strings.Contains(t, "function") {
			count++
		}
		count += s.estimateComplexity(child)
	}
	return count
}
