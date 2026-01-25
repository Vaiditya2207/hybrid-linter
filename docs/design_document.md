# System Design Document: Hybrid Linter

## 1. Introduction
This System Design Document provides the implementation details of the Hybrid Linter, moving beyond the high-level architecture to define the explicit data structures, algorithms, and Go patterns required to fulfill the SRS and Architecture specifications.

## 2. Core Modules and Packages
The system is divided into five primary Go packages to enforce separation of concerns:

- `pkg/scanner`: File system abstraction and initial Source Buffer ingestion.
- `pkg/parser`: Wrapper around `smacker/go-tree-sitter`. Handles full tree parsing and incremental tree editing.
- `pkg/analyzer`: Contains the Query Runner and `.scm` pattern definitions for vulnerability detection.
- `pkg/slicer`: The context extraction engine responsible for building the Syntax Independent Units (SIUs).
- `pkg/engine`: Handles FFI (Foreign Function Interface) bridging via `gollama.cpp` to execute the local LLM.
- `pkg/validator`: The Parse-Check-Retry loop managing feedback diagnostics.

## 3. Data Structures
### 3.1 The Vulnerability Profile (`analyzer.Vulnerability`)
When a node matches a `.scm` query, the system generates a `Vulnerability` record.
```go
type Vulnerability struct {
    ID          string
    Type        RuleType // e.g., UnhandledError, GoroutineLeak
    FocusNode   *sitter.Node
    StartByte   uint32
    EndByte     uint32
    FilePath    string
    SourceCode  []byte
}
```

### 3.2 The SIU Context (`slicer.SIU`)
The data passed into the SLM prompt is carefully constructed:
```go
type SIU struct {
    FocusContext []byte // The exact vulnerable code block
    Imports      []byte // Required dependencies
    StructDep    []byte // The parent struct definition
    Signatures   []byte // The skeleton of surrounding methods
}
```

## 4. Algorithmic Designs

### 4.1 The AST Slicing Algorithm
**Objective**: Build the `SIU` structure without exceeding 1024 tokens.
**Steps**:
1. Take `FocusNode`. Traversal Up stringifies the enclosing `type_declaration`.
2. Traversal Down collects `identifier` nodes. The system resolves these identifiers to their declaration nodes within the local scope using `locals.scm`.
3. Concatenate text streams. Validate token size using a fast Byte Pair Encoding (BPE) estimator.
4. If `tokens > 1024`, greedily stringify only function signatures instead of full blocks.

### 4.2 The "Parse-Check-Retry" Validation Loop
**Objective**: Ensure the SLM patch does not break parsing.
**Steps**:
1. Receive patch from `pkg/engine`.
2. Determine byte offset mapping of `FocusNode` in original file.
3. Slice `originalSrc` and insert `patch` to form `newSrc`.
4. Construct `sitter.InputEdit` matching the byte differences.
5. Invoke `oldTree.Edit(&edit)`.
6. Invoke `parser.ParseCtx(nil, newSrc)` with the edited old tree.
7. Perform DFS traversal checking for `node.Type() == "ERROR"` or `node.Type() == "MISSING"`.
8. If Error found, call `node.ToSexp()` to get the diagnostic string. Append to feedback string. Max 3 Retries.

## 5. Inference Bridging (CGO vs purego)
Given the strict sub-second performance requirement, the design enforces the use of pure Go wrappers targeting C-Shared libraries (`libllama.dylib`).
- We avoid `os/exec` to invoke CLI tools.
- We utilize `gollama.cpp` or a minimal custom CGO bridge to load the model into memory exactly once at startup. 
- The inference generation function acts as an asynchronous Go channel consumer.

## 6. Implementation of Quantization Settings
The application will bundle or download a pre-quantized GGUF artifact: `qwen2.5-coder-1.5b-instruct-q4_k_m.gguf`. 
The required configuration for context generation:
- `n_ctx`: 1024
- `n_predict`: 256
- `temperature`: 0.1 (We require deterministic, precise structural patches, not creative generation).

## 7. Extensibility Model
To allow users to expand the capabilities:
- Custom rules are defined as plain `.scm` text files in `~/.hybrid-linter/rules/`.
- The `Analyzer` package loads these at initialization. No recompilation of the linter is necessary to add new rule matching logic.
