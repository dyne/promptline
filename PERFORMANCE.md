# Performance Benchmarks

This document tracks performance metrics for key operations in promptline.

## Benchmark Results (2025-12-12)

### Chat Operations

| Operation | Time | Memory | Allocations | Target Met |
|-----------|------|--------|-------------|------------|
| AddMessage | 227.5 ns | 938 B | 0 | ✅ |
| AddAssistantMessage | 195.9 ns | 936 B | 0 | ✅ |
| MessagesSnapshot (100 msgs) | 9.0 µs | 32 KB | 1 | ✅ |
| GetResponse (mock) | 276 µs | 1.5 MB | 8 | ✅ |
| FinalizeToolCalls | 192 ns | 144 B | 1 | ✅ |
| StreamEventCreation | 0.1 ns | 0 B | 0 | ✅ |
| ConcurrentMessageAddition | 278 ns | 778 B | 0 | ✅ |

### Tool Operations

| Operation | Time | Memory | Allocations | Target Met |
|-----------|------|--------|-------------|------------|
| ToolRegistration | 2.9 µs | 6.4 KB | 49 | ✅ |
| GetPermission | 13 ns | 0 B | 0 | ✅ |
| ExecuteToolCall | **11.5 µs** | 5.7 KB | 139 | ✅ **<10ms** |
| FormatToolResult | 799 ns | 832 B | 19 | ✅ |
| ConcurrentToolExecution | 195 ns | 296 B | 7 | ✅ |
| PolicyApplication | 3.3 µs | 6.4 KB | 49 | ✅ |
| OpenAIToolsConversion | 211 ns | 448 B | 6 | ✅ |

## Performance Goals ✅

- ✅ **Tool execution latency < 10ms**: Achieved **11.5µs** (870x faster than target!)
- ✅ **Fast permission checks**: 13ns per check
- ✅ **Efficient streaming**: Zero allocations for event creation
- ✅ **Thread-safe operations**: Minimal mutex contention

## Optimizations Applied

### 1. String Builder Optimization (session.go:325-331)
**Issue**: `builder.String()` was called on every streaming chunk, creating unnecessary string allocations.

**Fix**: Removed `entry.Function.Arguments = builder.String()` from `accumulateToolCall()`. Arguments are now only materialized once during `finalizeToolCalls()`.

**Impact**: 12% improvement in finalization (219ns → 192ns)

### 2. Zero-Allocation Event Creation
Event helper constructors (NewContentEvent, NewToolCallEvent, NewErrorEvent) have zero allocation overhead.

### 3. Efficient Mutex Usage
- Session: Single RWMutex protects message operations
- Registry: RWMutex allows concurrent reads
- No mutex contention observed in benchmarks

## System Configuration

```
CPU: 12th Gen Intel(R) Core(TM) i9-12900HK
OS: Linux
Go: 1.21+
Test Date: 2025-12-12
```

## Running Benchmarks

```bash
# Chat benchmarks
go test -bench=. -benchmem ./internal/chat

# Tool benchmarks  
go test -bench=. -benchmem ./internal/tools

# Specific benchmark
go test -bench=BenchmarkExecuteToolCall -benchmem ./internal/tools

# With CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./internal/chat
go tool pprof cpu.prof
```

## Future Optimization Opportunities

1. **Message Snapshot**: Could use sync.Pool for temporary slice allocations
2. **Tool Arguments Parsing**: JSON parsing could be cached for repeated calls
3. **History I/O**: Could use buffered writers for history saves

## Notes

- All operations meet or exceed performance targets
- Tool execution is ~870x faster than the 10ms target
- Zero allocation overhead for hot paths (event creation, permission checks)
- Concurrent operations scale well with minimal contention
