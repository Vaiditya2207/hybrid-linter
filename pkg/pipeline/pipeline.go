package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Vaiditya2207/hybrid-linter/pkg/agent"
	"github.com/Vaiditya2207/hybrid-linter/pkg/analyzer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/engine"
	"github.com/Vaiditya2207/hybrid-linter/pkg/lsp"
	"github.com/Vaiditya2207/hybrid-linter/pkg/orchestrator"
	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
	"github.com/Vaiditya2207/hybrid-linter/pkg/scanner"
	"github.com/Vaiditya2207/hybrid-linter/pkg/slicer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/validator"
)

// Pipeline manages the concurrent execution of scanning and repairing.
type Pipeline struct {
	targetDir   string
	modelPath   string
	adjudicator *analyzer.Adjudicator
	analyzer    *analyzer.Analyzer // Baseline analyzer
	parser      *parser.Parser     // Baseline parser
}

type fileJob struct {
	path   string
	source []byte
}

type vulnJob struct {
	file  fileJob
	vulns []analyzer.Vulnerability
}

type repairJob struct {
	file fileJob
	vuln analyzer.Vulnerability
	fix  string
}

// NewPipeline initializes the concurrent architecture.
func NewPipeline(targetDir string, modelPath string, eng *engine.Engine) *Pipeline {
	var adj *analyzer.Adjudicator
	if eng != nil {
		adj = analyzer.NewAdjudicator(eng)
	}
	return &Pipeline{
		targetDir:   targetDir,
		modelPath:   modelPath,
		adjudicator: adj,
		analyzer:    analyzer.NewAnalyzer(),
		parser:      parser.NewParser(),
	}
}

// Run executes the pipeline with a given context.
func (p *Pipeline) Run(ctx context.Context) error {
	log.Println("Starting pipeline...")
	startTime := time.Now()

	fileChan := make(chan fileJob, 100)
	vulnChan := make(chan vulnJob, 100)

	// Phase 1: Scanner (Producer)
	var scannerWg sync.WaitGroup
	scannerWg.Add(1)
	go p.scanDirectory(ctx, fileChan, &scannerWg)

	// Layer 3: Optional Clangd LSP for C/C++ precision
	var lspClient *lsp.Client
	if _, err := exec.LookPath("clangd"); err == nil {
		lspClient, _ = lsp.NewClient("clangd")
		if lspClient != nil {
			absDir, _ := filepath.Abs(p.targetDir)
			_ = lspClient.Initialize(ctx, "file://"+absDir)
			defer lspClient.Close()
		}
	}

	// Phase 2: Analyzer Workers (Filter)
	const numAnalyzers = 4
	var analyzerWg sync.WaitGroup
	for i := 0; i < numAnalyzers; i++ {
		analyzerWg.Add(1)
		go p.analyzeFiles(ctx, fileChan, vulnChan, &analyzerWg, lspClient)
	}

	// Close vulnChan when analyzers are done
	go func() {
		analyzerWg.Wait()
		close(vulnChan)
	}()

	var allVulns []vulnJob
	for v := range vulnChan {
		allVulns = append(allVulns, v)
	}
	scannerWg.Wait()

	if len(allVulns) == 0 {
		log.Printf("\033[32mPipeline completed in %s. No vulnerabilities found.\033[0m", time.Since(startTime))
		return nil
	}

	log.Printf("\033[33mAnalysis complete. Found vulnerabilities in %d files. Proceeding to repair...\033[0m", len(allVulns))

	// Phase 3: Repair Workers
	repairChan := make(chan repairJob, 100)
	queuedVulns := make(chan vulnJob, len(allVulns))
	for _, v := range allVulns {
		queuedVulns <- v
	}
	close(queuedVulns)

	const numRepairers = 1
	var repairWg sync.WaitGroup
	for i := 0; i < numRepairers; i++ {
		repairWg.Add(1)
		go p.repairVulns(ctx, queuedVulns, repairChan, &repairWg)
	}

	go func() {
		repairWg.Wait()
		close(repairChan)
	}()

	// Phase 4: Writer (Consumer)
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go p.applyPatches(ctx, repairChan, &writerWg)

	writerWg.Wait()
	log.Printf("Pipeline completed in %s", time.Since(startTime))
	return nil
}

func (p *Pipeline) scanDirectory(ctx context.Context, fileChan chan<- fileJob, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(fileChan)

	scannerService := scanner.NewScanner([]string{".go", ".js", ".ts", ".py", ".c", ".cpp", ".rs", ".swift", ".zig"})
	out := make(chan scanner.FileResult, 100)

	var scanWg sync.WaitGroup
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		if err := scannerService.ScanDirectory(ctx, p.targetDir, out); err != nil {
			log.Printf("Scanner traversal error: %v", err)
		}
	}()

	for result := range out {
		fileChan <- fileJob{path: result.Path, source: result.Content}
	}
	scanWg.Wait()
}

func (p *Pipeline) analyzeFiles(ctx context.Context, fileChan <-chan fileJob, vulnChan chan<- vulnJob, wg *sync.WaitGroup, lspClient *lsp.Client) {
	defer wg.Done()
	
	for job := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
			ext := filepath.Ext(job.path)
			var localParser *parser.Parser
			var localAnalyzer *analyzer.Analyzer
			
			ruleName := analyzer.GetRuleForExtension(ext)
			queryData, err := analyzer.LoadEmbeddedQuery(ruleName)
			if err != nil {
				continue
			}

			switch ext {
			case ".c":
				localParser = parser.NewParserForC()
				localAnalyzer = analyzer.NewAnalyzerForC()
			case ".cpp", ".cc", ".cxx", ".h", ".hpp":
				localParser = parser.NewParserForCPP()
				localAnalyzer = analyzer.NewAnalyzerForCPP()
			default:
				localParser = parser.NewParser()
				localAnalyzer = analyzer.NewAnalyzer()
			}

			tree, err := localParser.Parse(ctx, job.source)
			if err != nil {
				continue
			}

			// Notify LSP of file content (Layer 3)
			if lspClient != nil && (ext == ".c" || ext == ".cpp" || ext == ".cc" || ext == ".h" || ext == ".hpp") {
				_ = lspClient.DidOpen(ctx, "file://"+job.path, "c", string(job.source))
			}

			voidFuncs := analyzer.BuildTypeMap(tree.RootNode(), job.source, filepath.Dir(job.path), 0)
			mustCheckFuncs := analyzer.ScanForMustCheck(tree.RootNode(), job.source)
			
			vulns, err := localAnalyzer.Analyze(ctx, tree.RootNode(), job.source, queryData, voidFuncs, mustCheckFuncs, lspClient, p.adjudicator, job.path)
			if err == nil && len(vulns) > 0 {
				vulnChan <- vulnJob{file: job, vulns: vulns}
			}
		}
	}
}

func (p *Pipeline) repairVulns(ctx context.Context, vulnChan <-chan vulnJob, repairChan chan<- repairJob, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.modelPath == "" {
		for range vulnChan {}
		return
	}

	if err := engine.InitBackend(); err != nil {
		log.Printf("Failed to init backend: %v", err)
		for range vulnChan {}
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	eng, err := engine.NewEngine(p.modelPath)
	if err != nil {
		log.Printf("Failed to init engine: %v", err)
		for range vulnChan {}
		return
	}
	defer eng.Close()

	slcr := slicer.NewSlicer(1024)
	vldr := validator.NewValidator(p.parser)
	orch := orchestrator.NewOrchestrator(p.analyzer, slcr, eng, vldr, 3)
	terminalAgent := agent.NewTerminalAgent(eng)

	for job := range vulnChan {
		for _, vuln := range job.vulns {
			select {
			case <-ctx.Done():
				return
			default:
				bugDesc := fmt.Sprintf("%s found at %s:%d", vuln.Type, job.file.path, vuln.StartLine)
				
				if os.Getenv("HYBRID_CHAT") == "1" {
					contextSIU, _ := slcr.ExtractContext(job.file.source, vuln.FocusNode)
					_, apply := terminalAgent.ChatLoop(ctx, contextSIU.String(), bugDesc)
					if !apply {
						continue
					}
				}

				fix, err := orch.RepairVulnerability(ctx, job.file.source, &vuln)
				if err == nil {
					repairChan <- repairJob{file: job.file, vuln: vuln, fix: fix}
				}
			}
		}
	}
}

func (p *Pipeline) applyPatches(ctx context.Context, repairChan <-chan repairJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range repairChan {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Printf("\033[32m✅ [FILE: %s] Repaired %s at line %d:\033[0m\n", job.file.path, job.vuln.Type, job.vuln.StartLine)
			fmt.Printf("\033[36m%s\033[0m\n\n", job.fix)
		}
	}
}
