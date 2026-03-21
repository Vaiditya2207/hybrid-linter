package analyzer

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Vaiditya2207/hybrid-linter/pkg/lsp"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
)

// Vulnerability represents a detected issue in the code.
type Vulnerability struct {
	ID        string
	Type      string
	Message   string
	StartLine uint32
	StartCol  uint32
	EndLine   uint32
	EndCol    uint32
	FocusNode *sitter.Node
}

// Analyzer runs SCM queries against a tree.
type Analyzer struct {
	Language *sitter.Language
}

// NewAnalyzer creates a new Go analyzer instance (default).
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		Language: golang.GetLanguage(),
	}
}

// NewAnalyzerForC creates a new C analyzer instance.
func NewAnalyzerForC() *Analyzer {
	return &Analyzer{
		Language: c.GetLanguage(),
	}
}

// NewAnalyzerForCPP creates a new CPP analyzer instance.
func NewAnalyzerForCPP() *Analyzer {
	return &Analyzer{
		Language: cpp.GetLanguage(),
	}
}

// Analyze runs the given SCM query against the root node of a tree.
func (a *Analyzer) Analyze(ctx context.Context, root *sitter.Node, source []byte, queryData []byte, voidFuncs map[string]bool, mustCheckFuncs map[string]bool, lspClient *lsp.Client, filePath string) ([]Vulnerability, error) {
	q, err := sitter.NewQuery(queryData, a.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, root)

	noiseRegex := regexp.MustCompile(`^(pr_|print|dev_|EXPORT_|MODULE_|__setup|late_init|core_init|postcore_init|arch_init|subsys_init|fs_init|device_init|pure_init|module_init|module_exit|WARN|BUG|panic|mutex_|spin_|raw_spin_|read_lock|read_unlock|write_lock|write_unlock|rcu_|debugfs_|trace_|lockdep_|smp_|cpu_|kfree|kmem_cache_free|free_page|vfree|put_device|put_task_struct|wait_event|wake_up|do_div|ktime_|memset|memcpy|memmove|strcpy|strcat|sprintf|snprintf|snprint|scnprintf|pr_cont|seq_printf|seq_puts)`)

	var vulnerabilities []Vulnerability
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		// Manual noise filtration
		skipMatch := false
		for _, cap := range m.Captures {
			captureName := q.CaptureNameForId(cap.Index)
			if captureName == "_func" {
				funcName := cap.Node.Content(source)
				
				// Layer 1 & 2: Void/Static checks
				if noiseRegex.MatchString(funcName) || voidFuncs[funcName] {
					skipMatch = true
					break
				}
				
				// Phase 27: CBP Whitelist
				// If we have a whitelist and this function isn't in it, we cautiously skip it
				// unless LSP (Layer 3) confirms it's a must-check.
				if mustCheckFuncs != nil && !mustCheckFuncs[funcName] {
					skipMatch = true 
				}

				// Layer 3: LSP Check (Precision override)
				if lspClient != nil && filePath != "" {
					fileURI := "file://" + filePath
					hover, err := lspClient.GetHover(ctx, fileURI, int(cap.Node.StartPoint().Row), int(cap.Node.StartPoint().Column))
					if err == nil && hover != "" {
						if strings.HasPrefix(hover, "void ") || strings.Contains(hover, "-> void") {
							skipMatch = true
							break
						} else {
							// If LSP definitely says it's NOT void, we ignore the CBP whitelist skip
							skipMatch = false
						}
					}
				}
			}
		}

		if skipMatch {
			continue
		}

		for _, cap := range m.Captures {
			captureName := q.CaptureNameForId(cap.Index)
			if len(captureName) > 0 && captureName[0] == '_' {
				continue
			}
			node := cap.Node

			// Phase 26: Data-Flow Check
			// If this is a variable assignment, check if it's handled
			if captureName == "err" {
				varName := node.Content(source)
				if IsVariableHandled(root, varName, node.StartPoint().Row, source) {
					continue
				}
			}

			vulnerabilities = append(vulnerabilities, Vulnerability{
				ID:        captureName,
				Type:      captureName,
				Message:   fmt.Sprintf("Detected %s", captureName),
				StartLine: node.StartPoint().Row + 1,
				StartCol:  node.StartPoint().Column + 1,
				EndLine:   node.EndPoint().Row + 1,
				EndCol:    node.EndPoint().Column + 1,
				FocusNode: node,
			})
		}
	}

	// Phase 28: Resource Leak Detection
	leaks := ScanForLeaks(root, source)
	vulnerabilities = append(vulnerabilities, leaks...)

	return vulnerabilities, nil
}

// LoadQueryFromFile loads a query from a file path.
func LoadQueryFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
