#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p benchmark_results
result="benchmark_results/benchmark_$(date +%Y%m%d_%H%M%S).txt"
{
  echo 'KiteCache local cache benchmarks'
  go version
  go test . -run '^$' -bench 'Benchmark(CacheGet|CacheSet|CacheMixed|LRU)' -benchmem -benchtime=3s -count=1 -cpu=1,2,4,8
  go test . -v -run '^(TestCacheHitRate|TestConcurrentStress)$' -count=1 -timeout=60s
} 2>&1 | tee "$result"
echo "Results: $result"
