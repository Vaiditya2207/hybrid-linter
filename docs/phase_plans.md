# Advanced Phase Plans: Low-Scale Model Implementation Roadmap

How to achieve the capabilities described in `adv_research.md` using local models like Qwen2.5-Coder-1.5B/3B on 8GB unified memory hardware.

---

## Phase 26: Intra-Procedural Data-Flow Tracking

**Goal**: Track whether a returned error variable is actually checked later in the function.

**What changes**: Instead of just flagging `err := doSomething()`, the linter will walk the CFG forward from the assignment and look for an `if err != nil` (Go) or `if (ret < 0)` (C) conditional. If it finds one, the issue is suppressed.

**Implementation (Pure Tree-sitter, No LLM needed)**:
1. Build a per-function Control-Flow Graph (CFG) from the AST using `pkg/analyzer/cfg.go`.
2. For each flagged vulnerability, walk the CFG forward from the assignment node.
3. Search for conditional expressions that reference the same variable name.
4. If found on ALL reachable paths -> suppress the issue.
5. If found on SOME paths -> downgrade severity to "Warning".
6. If found on NO paths -> keep as "High".

**Qwen Role**: None. This is purely deterministic AST logic.

**Estimated Effort**: 2-3 hours.
**Expected Impact**: Further 30-50% reduction in remaining issues.

---

## Phase 27: Callee Backtracking Prediction (CBP)

**Goal**: Automatically determine if a function's return value represents an error code, even without `__must_check` annotations.

**What changes**: Before analysis, the linter scans every function definition and looks backward from its `return` statements. If the function can return `-1`, `NULL`, `-ENOMEM`, or similar error sentinels, it is marked as "must-check". Functions that only return `0` or positive values are marked as "safe-to-ignore".

**Implementation (Tree-sitter + Simple Heuristics)**:
1. Create `pkg/analyzer/cbp.go`.
2. Walk all `function_definition` nodes.
3. For each, collect all `return_statement` nodes.
4. Extract the returned expression. Classify it:
   - Returns a negative literal or error macro -> "error-returning"
   - Returns NULL -> "nullable"
   - Returns only 0 or positive -> "safe"
   - Returns void -> "void" (already handled)
5. Store in a `mustCheckFuncs` map alongside the existing `voidFuncs` map.
6. In `Analyze`, only flag calls to functions in `mustCheckFuncs`.

**Qwen Role**: None. This is pure AST pattern matching.

**Estimated Effort**: 1-2 hours.
**Expected Impact**: Inverts the detection model (whitelist "must-check" instead of blacklist "void").

---

## Phase 28: Resource Leak Detection (Typestate Analysis)

**Goal**: Detect missing `unlock`, `free`, or `close` calls on error-return paths.

**What changes**: For each function, the linter builds a list of "acquired resources" (e.g., `mutex_lock`, `kmalloc`, `fopen`). It then verifies that every path to a `return` or `goto` statement passes through the corresponding "release" (e.g., `mutex_unlock`, `kfree`, `fclose`).

**Implementation (Tree-sitter CFG + Pair Matching)**:
1. Create `pkg/analyzer/typestate.go`.
2. Define resource pairs:
   ```
   mutex_lock     -> mutex_unlock
   spin_lock      -> spin_unlock
   kmalloc        -> kfree
   fopen          -> fclose
   ```
3. Walk the CFG. When an acquisition is seen, push it onto a "pending" stack.
4. When a release is seen, pop the matching acquisition.
5. At every function exit point (`return`, `goto err_out`), check if the pending stack is non-empty.
6. If a resource has no matching release on ANY path to exit -> flag as "Resource Leak".

**Qwen Role**: Used in `think` mode to explain the leak and suggest the correct cleanup label insertion.

**Estimated Effort**: 3-4 hours.
**Expected Impact**: Entirely new class of bugs detected (deadlocks, memory leaks).

---

## Phase 29: LLM-Guided Path Feasibility (Neural-Symbolic Light)

**Goal**: Use Qwen to prune false positives by asking "is this code path actually reachable?"

**What changes**: For each high-severity alert from Phases 26-28, the linter extracts the surrounding function context using the existing `Slicer` and asks Qwen a yes/no question:

> "Given this function, is the error path on line X reachable in normal operation? Answer YES or NO with a one-sentence justification."

If Qwen says "NO" with confidence, the alert is downgraded or suppressed.

**Implementation (Slicer + Qwen Prompt Engineering)**:
1. Create `pkg/analyzer/adjudicator.go`.
2. For each alert, extract context via `Slicer.ExtractContext()`.
3. Build a ChatML prompt with the code and the specific question.
4. Parse the response: `YES` keeps the alert, `NO` suppresses it.
5. Gate behind `--adjudicate` CLI flag (since it requires a model).

**Qwen Role**: Core. Acts as the "judge" for path feasibility.
**Token Budget**: ~200 tokens per alert (context + question + answer). On Qwen-1.5B, this is ~50ms per alert.

**Estimated Effort**: 1-2 hours (leverages existing Slicer + Engine).
**Expected Impact**: 20-40% further reduction in false positives.

---

## Phase 30: Microscopic Error-Handling Consistency

**Goal**: Detect inconsistent cleanup patterns within a single function's error paths.

**What changes**: When a function has multiple `goto` error labels (common in kernel C), the linter compares the cleanup operations at each label. If one label performs cleanup A, B, C but another only does A, B, the linter flags the missing C as a potential omission.

**Implementation (AST Pattern Matching)**:
1. Create `pkg/analyzer/ehc.go` (Error-Handling Consistency).
2. Find all `goto` statements in a function.
3. For each target label, collect the list of function calls made between the label and the next `return`.
4. Group labels by their "cleanup profile" (set of called functions).
5. If one label's profile is a strict subset of the majority profile -> flag as "Inconsistent Cleanup".

**Qwen Role**: Used in `think` mode to generate the missing cleanup call.

**Estimated Effort**: 2-3 hours.
**Expected Impact**: Catches a class of bugs that even `gcc -Wall` misses entirely.

---

## Phase 31: Git History Mining (BugStone-Light)

**Goal**: Learn project-specific bug patterns from historical commits.

**What changes**: The linter scans the last N git commits for patches that add error-checking code. It extracts the "rule" (which function call now has a check) and searches the rest of the codebase for violations of the same rule.

**Implementation (Git + Tree-sitter Diff Analysis)**:
1. Create `pkg/miner/miner.go`.
2. Run `git log --diff-filter=M -p --since=6months` to get recent patches.
3. Parse the diff hunks. Identify hunks where an `if (err)` check was ADDED around an existing function call.
4. Extract the function name as a "learned must-check rule".
5. Scan the entire codebase for calls to that function WITHOUT an error check.
6. Flag as "Historical Pattern: similar fix applied in commit <hash>".

**Qwen Role**: Summarize the learned rule in human-readable form ("This function was patched to include error checking in commit abc123").

**Estimated Effort**: 3-4 hours.
**Expected Impact**: Finds bugs that are duplicates of already-fixed issues elsewhere in the codebase.

---

## Priority and Dependency Order

| Phase | Name | Depends On | Qwen Required | New Bug Class |
| :--- | :--- | :--- | :--- | :--- |
| 26 | Data-Flow Tracking | None | No | Reduces false positives |
| 27 | Callee Backtracking | None | No | Inverts detection model |
| 28 | Resource Leak Detection | Phase 26 (CFG) | Optional | Deadlocks, Memory Leaks |
| 29 | LLM Path Feasibility | Phase 26 | Yes | Reduces false positives |
| 30 | EHC Consistency | Phase 26 (CFG) | Optional | Missing cleanup on error |
| 31 | Git History Mining | None | Optional | Recurring Pattern Bugs |

**Recommended start**: Phase 26 (CFG) and Phase 27 (CBP) can be done in parallel. They are purely deterministic and will immediately improve accuracy without needing any model at all.
