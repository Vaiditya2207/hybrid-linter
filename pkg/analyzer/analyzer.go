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
func (a *Analyzer) Analyze(ctx context.Context, root *sitter.Node, source []byte, queryData []byte, voidFuncs map[string]bool, mustCheckFuncs map[string]bool, lspClient *lsp.Client, adjudicator *Adjudicator, filePath string) ([]Vulnerability, error) {
	q, err := sitter.NewQuery(queryData, a.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, root)

	// Expansive noise filter: kernel functions whose return values are routinely discarded by design.
	// Categories: logging, lifecycle, synchronization, memory-free, reference counting, init macros,
	// tracing, scheduling, assertion, registration, notification, timer, workqueue, IRQ, mm, networking.
	noiseRegex := regexp.MustCompile(`^(` +
		// Logging and printing
		`pr_|print|dev_|dev_err|dev_warn|dev_info|dev_dbg|dev_notice|` +
		`EXPORT_|MODULE_|__setup|DEFINE_|DECLARE_|LIST_HEAD|` +
		// Init macros
		`late_init|core_init|postcore_init|arch_init|subsys_init|fs_init|device_init|pure_init|module_init|module_exit|` +
		// Assertions and panics
		`WARN|WARN_ON|WARN_ONCE|BUG|BUG_ON|panic|` +
		// Locking (acquire AND release - both return void)
		`mutex_lock|mutex_unlock|mutex_init|mutex_destroy|spin_lock|spin_unlock|raw_spin_lock|raw_spin_unlock|` +
		`read_lock|read_unlock|write_lock|write_unlock|rcu_read_lock|rcu_read_unlock|` +
		`down_read|down_write|up_read|up_write|rwlock_init|` +
		`spin_lock_init|spin_lock_irq|spin_unlock_irq|spin_lock_bh|spin_unlock_bh|` +
		`local_irq_save|local_irq_restore|local_irq_disable|local_irq_enable|` +
		`preempt_disable|preempt_enable|` +
		// Debug and tracing
		`debugfs_|trace_|lockdep_|ftrace_|` +
		// SMP and CPU
		`smp_|cpu_|per_cpu|get_cpu|put_cpu|` +
		// Memory free (never fail)
		`kfree|kvfree|vfree|free_page|free_pages|kmem_cache_free|__free_pages|` +
		`put_page|page_cache_release|folio_put|` +
		// Reference counting (return void or always succeed)
		`put_device|put_task_struct|fput|fdput|mntput|dput|iput|path_put|` +
		`kobject_put|kref_put|module_put|` +
		`get_device|get_task_struct|fget|igrab|` +
		`atomic_set|atomic_inc|atomic_dec|atomic_add|atomic_sub|` +
		`refcount_set|refcount_inc|refcount_dec|` +
		`kref_init|kref_get|` +
		// Scheduling and waiting
		`wait_event|wake_up|schedule|set_current_state|__set_current_state|` +
		`schedule_timeout|schedule_work|queue_work|queue_delayed_work|` +
		`complete|complete_all|init_completion|reinit_completion|` +
		// Math and string (always succeed or return into expressions)
		`do_div|min|max|clamp|abs|` +
		`memset|memcpy|memmove|memzero_explicit|` +
		`strcpy|strncpy|strlcpy|strcat|strncat|strlcat|` +
		`sprintf|snprintf|scnprintf|vsnprintf|` +
		// Seq file output
		`seq_printf|seq_puts|seq_putc|seq_write|pr_cont|` +
		// Notification and callback
		`notifier_chain_register|blocking_notifier_chain_register|` +
		`notifier_chain_unregister|` +
		// Timer
		`timer_setup|mod_timer|del_timer|del_timer_sync|add_timer|setup_timer|` +
		`hrtimer_init|hrtimer_start|hrtimer_cancel|` +
		// IRQ
		`enable_irq|disable_irq|free_irq|` +
		`irq_set_handler|irq_set_chip|` +
		// Misc void operations
		`INIT_LIST_HEAD|list_add|list_add_tail|list_del|list_del_init|list_move|list_move_tail|` +
		`INIT_WORK|INIT_DELAYED_WORK|` +
		`set_bit|clear_bit|test_and_set_bit|test_and_clear_bit|` +
		`sysfs_|` +
		`ktime_|` +
		`cond_resched` +
		`)`)

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

	/* High-Fidelity Phases (Disabled for stability - See /tmp/next_phases.md)
	// Phase 30: Microscopic Error-Handling Consistency (Hector EHC)
	ehcVulns := ScanForEHCInconsistencies(root, source)
	vulnerabilities = append(vulnerabilities, ehcVulns...)

	// Phase 31: Natural Language Specification (NLS) Documentation Auditing
	if adjudicator != nil {
		var walkNLS func(*sitter.Node)
		walkNLS = func(n *sitter.Node) {
			if n.Type() == "function_definition" {
				if findPrecedingComment(n, source) != "" {
					constraint, err := adjudicator.ExtractDocConstraints(ctx, n, source)
					if err == nil && constraint != nil {
						nlsVulns := a.VerifyNLSConstraints(n, source, constraint)
						vulnerabilities = append(vulnerabilities, nlsVulns...)
					}
				}
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walkNLS(n.Child(i))
			}
		}
		walkNLS(root)
	}

	// Phase 29: Neural Adjudication
	if adjudicator != nil && len(vulnerabilities) > 0 {
		var pruned []Vulnerability
		for _, v := range vulnerabilities {
			reachable, reason := adjudicator.Judge(ctx, source, &v)
			if reachable {
				pruned = append(pruned, v)
			} else {
				fmt.Printf("\033[35m[Neural Pruned] %s:%d: %s\033[0m\n", filePath, v.StartLine, reason)
			}
		}
		vulnerabilities = pruned
	}
	*/

	return vulnerabilities, nil
}

// LoadQueryFromFile loads a query from a file path.
func LoadQueryFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
