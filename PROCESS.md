# ⚙️ Working Process: Internal Architecture

Hybrid Linter follows a **tiered analysis pipeline** that separates deterministic structural verification from generative reasoning.

## 1. Structural Mapping (The SCM Layer)
The analyzer first maps the codebase into a Queryable Tree. We use **Tree-sitter** to execute Symbolic Code Mapping (SCM) queries.
- **Speed**: Scans ~100,000 lines in <10 seconds.
- **Precision**: Captures exact AST nodes (e.g., `(call_expression)` outcomes that discard `error` returns).

## 2. Context Extraction
Once a vulnerability is flagged, the tool extracts the surrounding source code, file metadata, and line numbers. This forms the "Context Frame."

## 3. Local Inference (The Local LLM Layer)
The Context Frame is injected into a strict **ChatML Template**.
- **Engine**: A custom CGO wrapper around `llama.cpp`.
- **Optimization**: We use 4-bit quantized GGUF weights (Q4_K_M) to fit high-reasoning models (3B+) into consumer laptop RAM.
- **Reliability**: Between every inference, the **KV Cache is Purged**. This resets the unified memory slots, preventing the GPU from "running out of breath" during large-scale batches.

## 4. Post-Processing & Repair
The LLM's output is detokenized (stripping BPE artifacts like `Ġ`) and validated.
- **Health Mode**: Formats the feedback into a structured diagnostic report.
- **Repair Mode**: Generates an AST patch and attempts to write the fix back to the source file.
