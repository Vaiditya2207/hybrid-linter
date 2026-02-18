package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Vaiditya2207/hybrid-linter/pkg/analyzer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/engine"
	"github.com/Vaiditya2207/hybrid-linter/pkg/orchestrator"
	"github.com/Vaiditya2207/hybrid-linter/pkg/parser"
	"github.com/Vaiditya2207/hybrid-linter/pkg/slicer"
	"github.com/Vaiditya2207/hybrid-linter/pkg/validator"
)

// Pipeline manages the concurrent execution of scanning and repairing.
type Pipeline struct {
	targetDir string
	analyzer  *analyzer.Analyzer
	modelPath string
	queryData []byte
	parser    *parser.Parser
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
func NewPipeline(targetDir string, a *analyzer.Analyzer, modelPath string, q []byte, p *parser.Parser) *Pipeline {
	return &Pipeline{
		targetDir: targetDir,
		analyzer:  a,
		modelPath: modelPath,
		queryData: q,
		parser:    p,
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

	// Phase 2: Analyzer Workers (Filter)
	const numAnalyzers = 4
	var analyzerWg sync.WaitGroup
	for i := 0; i < numAnalyzers; i++ {
		analyzerWg.Add(1)
		go p.analyzeFiles(ctx, fileChan, vulnChan, &analyzerWg)
	}

	// Close vulnChan when analyzers are done
	go func() {
		analyzerWg.Wait()
		close(vulnChan)
	}()

	// Collect all vulnerabilities sequentially to prevent Tree-sitter (CGO)
	// from overlapping with gollama.cpp (mmap) on macOS which causes SIGABRT.
	var allVulns []vulnJob
	for v := range vulnChan {
		allVulns = append(allVulns, v)
	}
	scannerWg.Wait()

	if len(allVulns) == 0 {
		log.Printf("Pipeline completed in %s. No vulnerabilities found.", time.Since(startTime))
		return nil
	}

	log.Printf("Analysis complete. Found vulnerabilities in %d files. Proceeding to repair...", len(allVulns))

	// Phase 3: Repair Workers
	repairChan := make(chan repairJob, 100)
	queuedVulns := make(chan vulnJob, len(allVulns))
	for _, v := range allVulns {
		queuedVulns <- v
	}
	close(queuedVulns)

	// We use 1 repair worker to prevent OOM on 8GB machines
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

	err := filepath.Walk(p.targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				source, err := os.ReadFile(path)
				if err == nil {
					fileChan <- fileJob{path: path, source: source}
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error scanning directory: %v", err)
	}
}

func (p *Pipeline) analyzeFiles(ctx context.Context, fileChan <-chan fileJob, vulnChan chan<- vulnJob, wg *sync.WaitGroup) {
	defer wg.Done()
	
	// Thread-safe parser instance for this specific worker goroutine
	localParser := parser.NewParser()

	for job := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
			tree, err := localParser.Parse(ctx, job.source)
			if err != nil {
				continue
			}

			vulns, err := p.analyzer.Analyze(ctx, tree.RootNode(), p.queryData)
			if err == nil && len(vulns) > 0 {
				vulnChan <- vulnJob{file: job, vulns: vulns}
			}
		}
	}
}

func (p *Pipeline) repairVulns(ctx context.Context, vulnChan <-chan vulnJob, repairChan chan<- repairJob, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.modelPath == "" {
		// Just drain the channel if no model is provided
		for range vulnChan {}
		return
	}

	// Wait for Phase 1 to completely finish! (Already guarded by sequential flow in Run, 
	// but we must initialize Backend_init only when we know Tree-sitter CGO is done).
	log.Printf("Initializing inference model backend (purego) on repair worker...")
	if err := engine.InitBackend(); err != nil {
		log.Printf("Failed to init backend: %v", err)
		for range vulnChan {}
		return
	}

	// Tie the inference engine to this specific OS thread to prevent Metal context 
	// crashes across goroutines.
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

	for job := range vulnChan {
		for _, vuln := range job.vulns {
			select {
			case <-ctx.Done():
				return
			default:
				fix, err := orch.RepairVulnerability(ctx, job.file.source, &vuln)
				if err == nil {
					repairChan <- repairJob{file: job.file, vuln: vuln, fix: fix}
				}
			}
		}
	}
}

// applyPatches receives fixes and writes them to the file cleanly via mutexes or sequential processing.
// Since this is the only consumer of repairChan, file system writes are naturally sequential here.
func (p *Pipeline) applyPatches(ctx context.Context, repairChan <-chan repairJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range repairChan {
		select {
		case <-ctx.Done():
			return
		default:
			// For Phase 5, we report the patch. Applying it cleanly to the byte array
			// requires calculating AST byte offsets, which falls into Phase 6 or further refinement.
			// MVP: just log the successful pipeline flow.
			fmt.Printf("✅ [FILE: %s] Repaired %s at line %d:\n%s\n", job.file.path, job.vuln.Type, job.vuln.StartLine, job.fix)
		}
	}
}
