# zerovllm: Project Context & Instructions

## Project Overview

**zerovllm** is a highly optimized IPC bridge designed to connect Go microservices or web agents directly to a local Python `vLLM` instance using ZeroMQ. It is specifically engineered for **heavy multi-modal (vision) workloads**, bypassing standard OpenAI-compatible REST APIs to eliminate bottlenecks caused by Base64 image encoding and JSON serialization CPU spikes.

### Key Technologies

- **Languages:** Go (Client), Python (Server/Router)
- **Communication:** ZeroMQ (`DEALER/ROUTER` sockets) over Unix Domain Sockets (`ipc://`)
- **Inference Engine:** vLLM (`AsyncLLMEngine`)
- **Telemetry:** OpenTelemetry (W3C context propagation)
- **License:** Apache 2.0

### Architecture

The project is split into two main components:

1. **Go Client (`client/`):** A thread-safe client that multiplexes requests from multiple goroutines onto a single ZeroMQ `DEALER` socket. It handles request/response matching via a `sync.Map`.
2. **Python Router (`zeroserver/`):** A `zmq.asyncio` loop that receives raw binary image frames and metadata, processing them via vLLM's `AsyncLLMEngine`.

## Building and Running

*Note: We use 2B models for testing to avoid memory pressure.*

### Prerequisites

- **ZeroMQ C Library:**
  - macOS: `brew install zeromq pkg-config`
  - Linux: `sudo apt-get install libzmq3-dev pkg-config`
- **Go:** 1.21+ (recommended)
- **Python:** 3.10+ with `uv` or `Poetry` for dependency management.

### Commands (Inferred from SPEC.md)

| Task | Command |
| :--- | :--- |
| **Go Tests** | `cd client && go test -v -race ./...` |
| **Python Tests** | `cd zeroserver && pytest -v tests/` |
| **Start Server** | `python -m zerovllm.server.main --model <model_name> --socket "ipc:///tmp/zerovllm.sock"` |
| **Go Linting** | `golangci-lint run` (TODO: verify config) |
| **Python Linting** | `ruff check .` (TODO: verify config) |

## Development Conventions

### General

- **Test-Driven Development (TDD):** The project emphasizes TDD for both Go and Python components.
- **IPC Protocol:**
  - Uses multipart ZeroMQ messages.
  - Frame 1: ZMQ Routing Identity (Internal to ZMQ).
  - Frame 2: JSON Metadata (Request ID, Prompt, Trace Context).
  - Frame 3-N: Raw binary image bytes (No Base64).

### Go Client (`client/`)

- **Concurrency:** Must be thread-safe for thousands of concurrent goroutines.
- **Memory:** Minimize allocations; pass `[]byte` by reference.
- **Context:** Always respect `context.Context` for timeouts and cancellation.

### Python Server (`zeroserver/`)

- **Asynchronous:** Use `zmq.asyncio` and vLLM's `AsyncLLMEngine`.
- **Error Handling:** Return structured JSON error frames for engine failures (e.g., Metal OOM).
- **Security:** UDS socket permissions should be restricted (e.g., `0600`).

## TDD Workflow & Commit Rules

### Before Writing Any Code

1. **Write the test first** - Test file must exist and fail
2. **Run the test** - Confirm it fails (RED state)
3. **Write minimal implementation** - Just enough to pass
4. **Run the test** - Confirm it passes (GREEN state)
5. **Refactor** - Improve code while keeping tests green
6. **Run all tests** - Ensure nothing broke
7. **Commit** - Test + implementation together

### Never Commit Without

- ✓ All tests passing
- ✓ 100% coverage of new code
- ✓ Cross-validation passing (if applicable)

## Key Files (Current)

- `README.md`: High-level project description and quick start.
- `SPEC.md`: Detailed technical specification for implementation.
- `LICENSE`: Apache 2.0 License.
- `.gitignore`: Standard Go/Python ignore patterns.

---
*Note: This file is used as foundational context for Gemini CLI. Adhere to the architecture and patterns described here when generating code.*
