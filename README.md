# Hybrid Linter: Local AI Code Diagnostics 🚀

A lightning-fast AST codebase analyzer and automated repair engine powered by Apple Metal-accelerated Local LLMs (via `gollama.cpp`). 

## Key Features
- **Deterministic SCM AST Queries**: Instantly find structural bugs, unhandled errors, and insecure definitions via Tree-sitter.
- **Local AI Context Processing (`think` mode)**: Seamlessly stream targeted LLM inferences to explicitly diagnose, explain, and automatically fix AST targets in pure markdown. 
- **Unified Global Cache**: Dynamically leverages Apple M-Series RAM buffering, guaranteeing zero-panic KV cache clearing during loops of hundreds of inferences!

## 📦 Installation

To globally install the `hybrid-linter` binary into your standard Go environment without any manual compiler overhead, run:
```bash
go install github.com/Vaiditya2207/hybrid-linter/cmd/hybrid-linter@latest
```

Ensure your `$(go env GOPATH)/bin` is added to your `$PATH`.

## 🤖 Model Setup

`hybrid-linter` utilizes quantized `.gguf` neural weights to infer solutions locally. This eliminates API fees and completely protects your proprietary source code offline. The tool natively downloads and caches the required GGUF weights locally to `~/.hybrid-linter/models`.

To seamlessly pull down the recommended **1.5B Qwen Coding Model**, simply type:
```bash
hybrid-linter install-model qwen-1.5b
```
*(Alternative support provided for `llama-3b`)*

## 🛠️ Usage Guides

### 1. Generating AI Health Reports
Pipe an entire codebase topology through the fast deterministic logic and invoke the LLM inference engine sequentially to stream root-cause bug summaries and proposed fixes:
```bash
hybrid-linter -dir . -health -mode think
```
*(Note: To run a static logic baseline without waking the LLM Engine, use `-mode fast`)*

### 2. Autonomous Local Bug Repair 
Attempt to leverage the contextual processor to completely rewrite malfunctioning file logic automatically:
```bash
hybrid-linter -dir /path/to/project -repair
```

### 3. Native Queries & Profiling
If you wish to overwrite the default `.scm` queries, you can supply your own AST rules natively:
```bash
hybrid-linter -dir . -query my_custom_rule.scm -memprofile out.prof
```

## Advanced Settings & CGO Environments
This application builds directly against raw C-pointers in MacOS via `gollama.cpp`. Because it strictly dynamically extracts its `dylib` pointers internally, you do not need `make` natively. However, note that inference loops scale based on available unified memory.
