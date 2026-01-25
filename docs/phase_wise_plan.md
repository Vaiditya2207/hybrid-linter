# Phase-wise Implementation Plan: Hybrid Linter

To ensure safe iteration and deterministic foundations before integrating probabilistic AI models, the development of the Hybrid Linter is divided into six logical phases.

## Phase 1: Foundation and SCM Query Mechanics
**Goal**: Establish the Go project structure and validate Tree-sitter's ability to parse Go code and run `.scm` queries.
1. Initialize Go modules and directory structure (`pkg/scanner`, `pkg/parser`, etc.).
2. Integrate `github.com/smacker/go-tree-sitter` and the `tree-sitter-go` language grammar.
3. Write initial unit tests to verify the AST generation of sample `.go` files.
4. Draft the three core SCM queries: `unhandled_errors.scm`, `goroutine_leaks.scm`, and `variable_shadowing.scm`.
5. Create a simple CLI command that outputs a JSON array of `Vulnerability` structs.

## Phase 2: AST Slicing and Context Manager
**Goal**: Implement the "Opaque Blob" generation to ensure contexts remain under 1,024 tokens.
1. Build the `pkg/slicer` module.
2. Implement specific upward traversals starting from a `FocusNode` to capture structural definitions (e.g., structs, interfaces).
3. Implement downward traversals using `locals.scm` to find reference typings.
4. Write the text generation logic that clips function bodies to signatures when tokens exceed limits.
5. Create unit tests feeding known vulnerable files and verifying the output SIU strings manually.

## Phase 3: Inference Bridge Integration
**Goal**: Establish a seamless FFI connection to `llama.cpp` using Metal API acceleration on M1 hardware.
1. Integrate `gollama.cpp` (or equivalent CGO/purego wrapper).
2. Download and side-load the `Qwen2.5-Coder-1.5B-Instruct-Q4_K_M.gguf` manifest into local storage.
3. Initialize the model instance strictly limiting configuration (`n_ctx=1024`, `n_gpu_layers=99`, `temp=0.1`).
4. Hardcode mock SIUs into a test loop and measure internal LLM Generation Latency (Tokens/sec).

## Phase 4: Deterministic Validation Loop
**Goal**: Build the Parse-Check-Retry protective barrier utilizing incremental tree editing.
1. Implement the `applyPatch` byte manipulation helper.
2. Connect `sitter.InputEdit` definitions to map byte ranges between original and patched code.
3. Implement `pkg/validator` containing the retry loop logic.
4. Write extensive tests by feeding intentionally broken generated code to the parser and asserting that `node.HasError()` properly short-circuits failure strings.
5. Implement the S-Expression extraction for the feedback context.

## Phase 5: Pipeline Orchestration
**Goal**: Connect all individual modules into the concurrent Filter-and-Pipe architecture.
1. Instantiate the Goroutine primitives (`Scanner`, `AnalysisQueue`, `RepairQueue`, `ValidationQueue`).
2. Map channels between components. Ensure graceful shutdown mechanisms and context cancellation on SIGINT.
3. Handle file-system locks to ensure patches are applied cleanly sequentially at the very last step.

## Phase 6: Final Polish and Developer Experience
**Goal**: Optimize UX and package the application for local developer environments.
1. Design CLI progress bars and colored TTY outputs separating detection (fast) and repair (variable speed).
2. Bundle default SCM rules.
3. Profile memory allocations to guarantee strict adherence to the < 8GB unified memory footprint.
