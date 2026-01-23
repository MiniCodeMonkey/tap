# Tap Performance Baselines

This document describes the performance targets and benchmarking methodology for Tap.

## Performance Targets

| Operation | Target | Description |
|-----------|--------|-------------|
| Parse 100 slides | < 100ms | Time to parse a 100-slide markdown presentation |
| Build 50 slides | < 2s | Time to generate static files for a 50-slide presentation |
| Hot reload latency | < 200ms | Time from file change detection to WebSocket broadcast |

## Running Benchmarks

### All Benchmarks

```bash
make bench
```

### Package-Specific Benchmarks

```bash
# Parser benchmarks
make bench-parser

# Builder benchmarks
make bench-builder

# Server benchmarks (includes hot reload)
make bench-server
```

### Performance Target Tests

Run tests that verify performance targets are met:

```bash
make bench-targets
```

This runs all tests with names containing "PerformanceTarget".

### Detailed Benchmark Output

For detailed output including memory allocations:

```bash
go test -bench=. -benchmem -benchtime=10s ./internal/parser/
```

## Benchmark Descriptions

### Parser Benchmarks

| Benchmark | Description |
|-----------|-------------|
| `BenchmarkParse100Slides` | Parse a realistic 100-slide presentation with various slide types |
| `BenchmarkParse50Slides` | Parse a 50-slide presentation |
| `BenchmarkParse200Slides` | Parse a larger 200-slide presentation |
| `BenchmarkParseWithManyCodeBlocks` | Parse slides with many code blocks (2 per slide) |
| `BenchmarkParseWithManyFragments` | Parse slides with many fragments (10 per slide) |
| `BenchmarkParserNew` | Create a new parser instance |
| `BenchmarkParseDirectives` | Parse slide directives in isolation |
| `BenchmarkParseCodeBlocks` | Parse code blocks in isolation |
| `BenchmarkParseFragments` | Parse fragments in isolation |

### Builder Benchmarks

| Benchmark | Description |
|-----------|-------------|
| `BenchmarkBuild50Slides` | Build static files for 50 slides |
| `BenchmarkBuild100Slides` | Build static files for 100 slides |
| `BenchmarkBuild200Slides` | Build static files for 200 slides |
| `BenchmarkGenerateIndexHTML` | Generate index.html with embedded JSON |
| `BenchmarkExtractImagePaths` | Extract image paths from HTML |
| `BenchmarkRewriteImagePaths` | Rewrite image paths using path mapping |
| `BenchmarkIsAbsoluteURL` | Check if URLs are absolute |

### Server Benchmarks

| Benchmark | Description |
|-----------|-------------|
| `BenchmarkHotReloadLatency` | Full hot reload cycle latency |
| `BenchmarkServerRoutePerformance` | HTTP route handling performance |
| `BenchmarkWebSocketHubBroadcast` | WebSocket hub message broadcasting |
| `BenchmarkWatcherDebounce` | File watcher debounce logic |
| `BenchmarkPresentationJSON` | JSON serialization of presentations |
| `BenchmarkServerStartStop` | Server startup and shutdown |

## Interpreting Results

Benchmark output format:

```
BenchmarkParse100Slides-8    1234    850000 ns/op    456789 B/op    1234 allocs/op
```

- `-8`: Number of CPU cores
- `1234`: Number of iterations
- `850000 ns/op`: Nanoseconds per operation (0.85ms)
- `456789 B/op`: Bytes allocated per operation
- `1234 allocs/op`: Memory allocations per operation

## Baseline Measurements

The following baseline measurements were established during development:

### Parser Performance

| Operation | Expected Range |
|-----------|---------------|
| Parse 100 slides | 10-50ms |
| Parse 200 slides | 20-100ms |
| Parse directives | 1-5μs |
| Parse code blocks | 2-10μs |
| Parse fragments | 1-5μs |

### Builder Performance

| Operation | Expected Range |
|-----------|---------------|
| Build 50 slides | 50-500ms |
| Build 100 slides | 100-1000ms |
| Generate index.html | 1-10ms |

### Server Performance

| Operation | Expected Range |
|-----------|---------------|
| Hot reload latency | 50-150ms |
| Route handling | 10-100μs |
| JSON serialization | 100-500μs |

## Regression Testing

To catch performance regressions:

1. Run benchmarks before and after changes:

```bash
# Before changes
go test -bench=. -benchmem ./... > bench_before.txt

# After changes
go test -bench=. -benchmem ./... > bench_after.txt
```

2. Compare results using `benchstat`:

```bash
go install golang.org/x/perf/cmd/benchstat@latest
benchstat bench_before.txt bench_after.txt
```

## Optimizing Performance

### Parser Optimizations

- Compiled regex patterns are reused (package-level vars)
- Content is processed as strings to minimize allocations
- Slide slices are pre-allocated with estimated capacity

### Builder Optimizations

- Content hashing uses SHA256 (fast and collision-resistant)
- Image path mapping avoids redundant processing
- HTML is generated with minimal string concatenation

### Server Optimizations

- File watcher uses debouncing to coalesce rapid changes
- WebSocket hub uses channels for thread-safe broadcasting
- HTTP routes use standard library for optimal performance

## Continuous Integration

Performance tests can be run in CI to catch regressions:

```yaml
# Example GitHub Actions step
- name: Run benchmarks
  run: make bench-targets
```

Consider adding benchmark result tracking to your CI pipeline for trend analysis.
