@echo off
setlocal
cd /d "%~dp0"
if not exist benchmark_results mkdir benchmark_results
set "RESULT_FILE=benchmark_results\benchmark_%RANDOM%_%RANDOM%.txt"
echo KiteCache local cache benchmarks > "%RESULT_FILE%"
go version >> "%RESULT_FILE%" 2>&1
go test . -run "^$" -bench "Benchmark(CacheGet|CacheSet|CacheMixed|LRU)" -benchmem -benchtime=3s -count=1 -cpu=1,2,4,8 >> "%RESULT_FILE%" 2>&1
if errorlevel 1 goto failed
go test . -v -run "^(TestCacheHitRate|TestConcurrentStress)$" -count=1 -timeout=60s >> "%RESULT_FILE%" 2>&1
if errorlevel 1 goto failed
type "%RESULT_FILE%"
echo Results: %RESULT_FILE%
exit /b 0
:failed
type "%RESULT_FILE%"
exit /b 1
