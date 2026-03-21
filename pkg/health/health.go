package health

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Vaiditya2207/hybrid-linter/pkg/analyzer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/lsp"
	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
	"github.com/Vaiditya2207/hybrid-linter/pkg/scanner"
	sitter "github.com/smacker/go-tree-sitter"
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
type Scorer struct{}

func NewScorer() *Scorer {
	return &Scorer{}
}

// GenerateScore walks a directory using the Phase 7 scanner and computes metrics.
func (s *Scorer) GenerateScore(ctx context.Context, dir string) (*CodebaseHealth, error) {
	start := time.Now()
	health := &CodebaseHealth{
		FileDistribution: make(map[string]int),
	}

	// Use our new Phase 7 Scanner to skip node_modules/venv etc.
	scan := scanner.NewScanner([]string{".go", ".c", ".cc", ".cpp", ".py", ".ts", ".js", ".swift", ".zig"})
	fileChan := make(chan scanner.FileResult, 100)

	// Layer 3: Optional Clangd LSP
	var lspClient *lsp.Client
	if _, err := exec.LookPath("clangd"); err == nil {
		lspClient, _ = lsp.NewClient("clangd")
		if lspClient != nil {
			absDir, _ := filepath.Abs(dir)
			_ = lspClient.Initialize(ctx, "file://"+absDir)
			defer lspClient.Close()
		}
	}

	go func() {
		scan.ScanDirectory(ctx, dir, fileChan)
	}()

	for file := range fileChan {
		health.TotalFiles++
		ext := strings.ToLower(filepath.Ext(file.Path))
		health.FileDistribution[strings.TrimPrefix(ext, ".")]++

		lines := strings.Count(string(file.Content), "\n")
		health.TotalLines += lines

		var p *parser.Parser
		var a *analyzer.Analyzer
		
		ruleName := analyzer.GetRuleForExtension(ext)
		queryData, err := analyzer.LoadEmbeddedQuery(ruleName)
		if err != nil {
			continue
		}

		switch ext {
		case ".c":
			p = parser.NewParserForC()
			a = analyzer.NewAnalyzerForC()
		case ".cpp", ".cc", ".cxx", ".h", ".hpp":
			p = parser.NewParserForCPP()
			a = analyzer.NewAnalyzerForCPP()
		default:
			p = parser.NewParser()
			a = analyzer.NewAnalyzer()
		}

		tree, err := p.Parse(ctx, file.Content)
		if err != nil {
			continue
		}

		// Build type maps for filtering
		voidFuncs := analyzer.BuildTypeMap(tree.RootNode(), file.Content, filepath.Dir(file.Path), 0)
		mustCheckFuncs := analyzer.ScanForMustCheck(tree.RootNode(), file.Content)

		// Notify LSP of file content (Layer 3)
		if lspClient != nil && (ext == ".c" || ext == ".cpp" || ext == ".cc" || ext == ".h" || ext == ".hpp") {
			_ = lspClient.DidOpen(ctx, "file://"+file.Path, "c", string(file.Content))
		}

		// Count unhandled errors as a primary health metric
		vulns, err := a.Analyze(ctx, tree.RootNode(), file.Content, queryData, voidFuncs, mustCheckFuncs, lspClient, file.Path)
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
