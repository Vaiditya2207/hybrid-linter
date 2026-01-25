# Software Requirements Specification (SRS) for Hybrid Linter

## 1. Introduction
### 1.1 Purpose
This document specifies the software requirements for the "Hybrid Linter", a high-performance, locally executable automated code repair tool for Go. It details the functional, performance, and architectural constraints required guiding its development.

### 1.2 Scope
The Hybrid Linter is designed to operate seamlessly on consumer-grade hardware (specifically 8GB Apple M1 machines) to detect complex logical bugs and provide verifiable, syntactically correct remediations using Small Language Models (SLMs). The tool merges deterministic Abstract Syntax Tree (AST) analysis via Tree-sitter with probabilistic inference.

### 1.3 Definitions and Acronyms
- **AST**: Abstract Syntax Tree.
- **CST**: Concrete Syntax Tree.
- **SLM**: Small Language Model (e.g., 1B-3B parameters).
- **SIU**: Syntax Independent Unit.
- **TTFT**: Time to First Token.

## 2. Overall Description
### 2.1 Product Perspective
The Hybrid Linter operates as a standalone CLI tool or IDE backend, running entirely on the developer's local machine to ensure privacy and zero network latency. It interfaces with the file system for Go source code and utilizes local Metal GPU acceleration via `llama.cpp`.

### 2.2 Product Functions
1. **Repository Scanning**: Rapid structural parsing of Go codebases.
2. **Issue Detection**: Identification of unhandled errors, goroutine leaks, and variable shadowing using highly specialized SCM queries.
3. **Context Slicing**: Extraction of <1024 token contexts around vulnerable nodes, retaining semantic dependencies (Structs, Imports, Signatures).
4. **Code Repair**: Proposition of code fixes using a quantized SLM (Qwen2.5-Coder-1.5B-Instruct).
5. **Deterministic Validation**: In-memory "Parse-Check-Retry" loop ensuring the SLM output is syntactically valid before presenting to the user.

### 2.3 User Characteristics
The target users are Go software engineers requiring immediate, localized, intelligent feedback and automated remediations during active development without comprising proprietary code.

### 2.4 Operating Environment
- **Hardware Profile**: Minimum 8GB Unified Memory, Apple M1/M2/M3 Silicon.
- **OS**: macOS.
- **Language**: Go 1.21+.

### 2.5 Design Constraints
- Must avoid heavy HTTP-based wrappers (e.g., Ollama APIs) in favor of direct CGO or `purego` bindings to `llama.cpp`.
- Model footprint must fit within the 8GB shared memory architecture (requiring 4-bit GGUF quantization).
- Extreme quantization (e.g., 1.5-bit) is prohibited due to reasoning degradation.

## 3. Specific Requirements
### 3.1 External Interface Requirements
- **CLI Interface**: Standard POSIX-compliant CLI flags.
- **Tree-sitter Binding**: Integration with `github.com/smacker/go-tree-sitter`.
- **LLM Engine**: Integration with `llama.cpp` using Metal API backends.

### 3.2 Functional Requirements
- **FR1 - SCM Queries**: The system shall execute Tree-sitter SCM queries to flag problematic nodes.
- **FR2 - AST Slicing**: The system shall perform upward/downward tree traversals to generate SIUs <= 1024 tokens.
- **FR3 - Generation**: The system shall prompt the local model and capture the output stream.
- **FR4 - Validation Loop**: The system shall apply the generated patch to an in-memory CST copy.
- **FR5 - Diagnostic Feedback**: If a patch is structurally invalid, the system shall convert the failed Tree-sitter ERROR node to an S-expression string and reprompt the model for maximum 3 retries.

### 3.3 Performance Requirements
- **Latency**: Sub-second latency for the entire analysis-to-repair cycle.
- **Generation Speed**: Minimum 120 tokens/second on M1 hardware.
- **Memory Overhead**: Total memory utilization (OS + Runtime + Model) shall not exceed 8GB. Model footprint ~1.0GB (4-bit Qwen2.5-Coder-1.5B).
- **AST Parsing**: Incremental updates must complete in <1ms.

### 3.4 Software System Attributes
- **Concurrency**: The tool will utilize a multi-stage goroutine-based pipeline to decouple I/O parsing from GPU-bound inference.
- **Privacy**: Zero external API calls. All code remains strictly on-device.
