# System Architecture Document: Hybrid Linter

## 1. Architectural Overview
The Hybrid Linter is structurally designed as a multi-stage concurrent pipeline running entirely on the developer's local machine. It orchestrates a formal grammar engine (Tree-sitter) with a probabilistic inference engine (Qwen2.5-Coder-1.5B via `llama.cpp`) to achieve high-accuracy automated code repair without the latency and privacy concerns of cloud-based APIs.

The core architecture follows a **Filter-and-Pipe** pattern where data (source code) strictly flows through sequential processing units: Scanning, Analysis, Repair, and Validation.

### 1.1 High-Level Context
The system bridges Go's file I/O and Tree-sitter's C-based parsing with the Metal-accelerated inference backend of `llama.cpp`.

![Hybrid Linter Core Architecture](./diagrams/hybrid_linter_architecture.png)

## 2. Component Design & Pipeline Flow
The architecture achieves its sub-second latency target by aggressively decoupling I/O-bound parsing from GPU-bound inference using Go concurrency primitives (Channels and Goroutines).

### 2.1 The Data Flow Pipeline
1. **Scanner Stage**: Continuously monitors the file system for changed `.go` files or performs a bulk read. Submits buffers to the Tree-sitter C bindings.
2. **Analysis Stage**: Executes Lisp-like `.scm` queries against the generated Concrete Syntax Tree (CST) to flag semantic or logical violations (e.g., Goroutine leaks).
3. **AST Slicing Mechanism**: Isolates the "Focus Node" and resolves semantic dependencies (Types, Imports) by traversing the AST up and down. Outputs a Syntax Independent Unit (SIU) strictly under 1,024 tokens.
4. **Repair Stage (llama.cpp integration)**: Connects to the local model via CGO/`purego` bindings to generate a code patch.
5. **Validation Loop**: Intercepts the generated text from the model and applies it to a virtual, in-memory copy of the AST using the `tree.Edit` API. If the parse results in `ERROR` nodes, it generates a precise S-Expression diagnostic and re-prompts the model.

![Pipeline Data Flow](./diagrams/pipeline_flow.png)

## 3. Subsystem Breakdown

### 3.1 Syntax Independent Unit (SIU) Extractor
To bypass the memory bottlenecks of the 8GB M1 architecture, the SIU generator acts as a context miniaturizer. Rather than performing simple regex-based line extractions, it constructs a functionally isolated but structurally sound Go fragment.

![AST Slicing Protocol](./diagrams/ast_slicing.png)

#### SIU Pruning Algorithm
If the merged context exceeds the 1,024-token limit, the system initiates an aggressive pruning strategy:
- Drop function bodies of adjacent struct methods, retaining only the signatures.
- Drop unused imports.
- Truncate nested blocks that do not intersect with the Focus Node's variable scope.

### 3.2 The Deterministic Validation Controller
The Validation Logic sits directly between the Model Output and the File System. It provides absolute guarantees that the Hybrid Linter will never write syntactically invalid code to the disk.

By leveraging `tree.Edit`, the controller updates the AST incrementally for speed (<1ms). The structural validation loop acts as a compiler-frontend check, utilizing `node.HasError()` as the failure condition.

## 4. Hardware and Performance Profiles
- **Memory Allocation Strategy**: The Apple Silicon Unified Memory is explicitly segmented. ~1.0GB is strictly reserved for the 4-bit Quantized GGUF Model. The Go runtime, TS parser, and OS share the remaining 7GB.
- **Inference Server**: Discards high-overhead REST APIs (e.g., Ollama) to achieve direct execution over the Metal API (up to 160 tokens/second).

## 5. Security & Privacy
The architectural boundary strictly isolates the repair process to the `localhost`. No network sockets are opened for external data transmission, ensuring 100% code privacy. Model updates or telemetry are entirely disabled by default.
