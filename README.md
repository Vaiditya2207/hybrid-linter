# 🛠️ Hybrid Linter

### **Local-First AI Codebase Diagnostics & Autonomous Repair**
[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-macOS%20(Metal)-orange.svg)](https://developer.apple.com/metal/)

**Hybrid Linter** is a next-generation static analysis tool that bridges the gap between **deterministic AST queries** and **agentic AI reasoning**. It utilizes Tree-sitter for lightning-fast structural bug detection and locally-hosted LLMs for contextual diagnostics and automated repair.

---

## 🌟 Key Features

- **🚀 Metal-Accelerated Inference**: Native Apple Silicon support via `gollama.cpp` for ultra-fast local LLM execution.
- **🧠 Agentic "Think" Mode**: Context-aware diagnostics that explain *why* a bug exists and *how* to fix it.
- **🔄 Unified Memory Management**: Custom KV Cache flushing enables continuous analysis of 500+ issues on standard 8GB/16GB hardware.
- **🛡️ 100% Private & Offline**: No API calls, no data leakage. Your source code never leaves your machine.
- **🏗️ Tree-Sitter Core**: Uses Symbolic Code Mapping (SCM) for sub-millisecond structural pattern matching.

---

## 📦 Installation

Install the global CLI binary directly from source:

```bash
go install github.com/Vaiditya2207/hybrid-linter/cmd/hybrid-linter@latest
```

---

## 🤖 Getting Started

### 1. Initialize the AI Environment
Pull the optimized 1.5B parameter coding model into your global cache:
```bash
hybrid-linter install-model qwen-1.5b
```

### 2. Generate a "Think" Mode Report
Run a deep-context diagnostic on any project directory:
```bash
hybrid-linter -dir /path/to/project -health -mode think
```

### 3. Trigger Autonomous Repair
Automatically patch unhandled errors and AST vulnerabilities:
```bash
hybrid-linter -dir . -repair
```

---

## 📝 Documentation & Results

For a deep dive into the internal mechanics and real-world performance metrics:

- [**Working Process (Architecture)**](PROCESS.md) - How the Hybrid Pipeline works.
- [**Case Study: okernel Stress Test**](RESULTS.md) - Performance benchmarks and AI results.

---

## ⚖️ License
Distributed under the MIT License. See `LICENSE` for more information.
