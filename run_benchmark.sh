#!/bin/bash

echo "Starting Hybrid-Linter Benchmark..."

mkdir -p results
START_TIME=$(date +%s)

go run ./cmd/hybrid-linter -dir ./benchmark -repair -model ./models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf > results/benchmark_out.txt

END_TIME=$(date +%s)
echo "Benchmark completed in $(($END_TIME - $START_TIME)) seconds."
echo "Results saved to results/benchmark_out.txt"

VULNS_FOUND=$(grep "Analysis complete. Found vulnerabilities" results/benchmark_out.txt | awk '{print $5}')
REPAIRS_MADE=$(grep -c "Repaired short_var_declaration" results/benchmark_out.txt)

echo "======================================"
echo "          BENCHMARK RESULTS           "
echo "======================================"
echo "Files Analyzed: 3"
echo "Vulnerabilities Found: $VULNS_FOUND"
echo "Successfully Repaired: $REPAIRS_MADE"
echo "Time Taken: $(($END_TIME - $START_TIME))s"
echo "======================================"

cat results/benchmark_out.txt
