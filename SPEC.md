Here is the finalized, production-ready specification, incorporating all the QA corrections (constructors, Python testing, backpressure, strict dependency management, and memory constraints).

I have formatted it as a complete Markdown code block so you can easily copy it and save it as a `.md` file for your team's repository.

```markdown
# Final Comprehensive Specification: ZeroMQ-vLLM Multi-Modal IPC Client

## 1. System Overview
This specification details the architecture, data structures, and implementation guidelines for a high-performance, asynchronous Inter-Process Communication (IPC) bridge between a Go-based web agent (or generic service) and a Python `vLLM` inference backend. 

By utilizing ZeroMQ's `DEALER/ROUTER` sockets over Unix domain sockets (`ipc://`), the system bypasses HTTP overhead and Base64 encoding, enabling highly concurrent, zero-network-latency multi-modal AI inference optimized for macOS (Apple Silicon/Metal).



---

## 2. High-Level Architecture

The system consists of two decoupled components communicating via a ZeroMQ IPC socket:

1. **Go Client (`vllmipc`):** Uses a `DEALER` socket multiplexed across multiple goroutines. It validates payloads, handles timeouts, propagates OpenTelemetry (OTel) context, and routes responses back to the correct caller asynchronously.
2. **Python Router (`vllm_router`):** Uses a `zmq.asyncio.ROUTER` socket to receive multipart messages concurrently. It reconstructs multi-image payloads in memory, executes inference via `vLLM`'s `AsyncLLMEngine`, and returns the results to the exact Go caller using the ZMQ routing identity.

---

## 3. Detailed Module Specifications

### 3.1 Go Client Module (`vllmipc`)

**Responsibility:** Provide a thread-safe, generic interface for Go services to send multi-modal requests to vLLM.

#### Structs & Interfaces

```go
package vllmipc

import (
	"context"
	"time"
)

// Config defines connection details and payload constraints.
type Config struct {
	SocketPath         string        // e.g., "ipc:///tmp/vllm.sock"
	MaxImages          int           // e.g., 6
	MaxImageSizeBytes  int64         // e.g., 5 * 1024 * 1024 (5MB)
	RequestTimeout     time.Duration // Max wait time for LLM response
	ZMQHighWaterMark   int           // Backpressure threshold for DEALER socket (e.g., 1000)
}

// Client represents the thread-safe ZeroMQ client.
type Client interface {
	GenerateVision(ctx context.Context, prompt string, images [][]byte) (*VisionResponse, error)
	Close() error
}

// NewClient initializes the ZMQ context, applies the HWM, binds the DEALER socket, 
// and spawns the background reader/writer multiplexer goroutines.
func NewClient(cfg Config) (Client, error)

// VisionMetadata is serialized to JSON as the first application frame.
type VisionMetadata struct {
	RequestID    string            `json:"request_id"`
	Prompt       string            `json:"prompt"`
	TraceContext map[string]string `json:"trace_context"` // OpenTelemetry W3C propagation
}

// VisionResponse is the expected JSON reply payload.
type VisionResponse struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
}

```

#### Data Flow

1. **Validation:** `GenerateVision` validates `len(images)` and byte sizes against `Config`. Returns `ErrConstraintViolated` immediately on failure.
2. **Context & Routing:** Extracts OTel trace data, generates a UUID (`RequestID`), and maps a response channel (`chan *VisionResponse`) in a `sync.Map`.
3. **Dispatch:** Pushes `VisionMetadata` and `[][]byte` to the ZMQ writer goroutine.
4. **Wait/Timeout:** Blocks on `select { case <-ch: ... case <-ctx.Done(): ... }`. Cleans up the `sync.Map` entry regardless of the outcome to prevent memory leaks.

### 3.2 Python Backend Module (`vllm_router`)

**Responsibility:** Manage the ZMQ `ROUTER` socket, handle backpressure, and execute local inference safely.

#### Data Flow & Error Handling

1. **Receive:** Awaits multipart frames: `[ZMQ_Identity, Metadata_JSON, Image_1_Bytes, ..., Image_N_Bytes]`.
2. **Telemetry Extraction:** Parses `TraceContext` to start an OTel span linked to the Go service's trace.
3. **Image Loading:** Iterates over binary frames, wrapping them in `io.BytesIO` and loading via `PIL.Image`.
4. **Inference Execution:** Passes images to `AsyncLLMEngine`.
5. **Failure States & OOM:**
* If `AsyncLLMEngine` fails (e.g., Metal OOM), catch the exception and return a structured JSON error frame: `{"request_id": "...", "error": "Inference failed: Metal OOM"}`.
* Configure the ZMQ `ROUTER` socket with a strict `SNDHWM` and `RCVHWM` (High Water Mark). If the queue is full, ZMQ will drop messages, triggering a timeout on the Go side rather than crashing the Python process.



---

## 4. File and Directory Structure

```text
zerovllm/
├── client/
│   ├── go.mod
│   ├── client.go           # Interface, Config, NewClient constructor
│   ├── multiplexer.go      # sync.Map routing and DEALER socket loops
│   ├── telemetry.go        # OTel trace injection
│   ├── errors.go           # Custom error types
│   └── client_test.go      # TDD tests
└── zeroserver/
    ├── pyproject.toml      # Strict dependencies (uv or Poetry)
    ├── uv.lock             # Reproducible lockfile
    ├── src/
    │   ├── main.py         # Entry point, CLI args parsing
    │   ├── router.py       # ZeroMQ ROUTER socket loop
    │   ├── inference.py    # vLLM AsyncLLMEngine wrapper
    │   └── telemetry.py    # OTel span extraction logic
    └── tests/
        ├── conftest.py     # Pytest fixtures
        └── test_router.py  # Async routing tests
```

---

## 5. Testing Strategy (TDD)

### 5.1 Go Client Testing

* **Unit Tests:** * Assert `NewClient` correctly applies config limits.
* Assert `GenerateVision` rejects payloads exceeding `MaxImages` or `MaxImageSizeBytes` without accessing ZMQ.
* Assert context timeouts correctly clean up the `sync.Map` and return `context.DeadlineExceeded`.


* **Integration Tests:** * Mock a Python `ROUTER` server in Go.
* Spawn 100+ goroutines making concurrent requests. Assert no deadlocks occur and the multiplexer correctly routes responses back to the right caller using `RequestID`.



### 5.2 Python Backend Testing (`pytest`)

* **Socket Logic:** Decouple the ZMQ socket loop from the `AsyncLLMEngine`. Use `pytest-asyncio` to send mock multipart frames over a test IPC socket and assert the Python server replies with the correct ZMQ routing identity and valid JSON.
* **Mock Engine:** Mock `vllm.engine.async_llm_engine.AsyncLLMEngine` to simulate successful generation and simulated OOM exceptions. This ensures the CI/CD pipeline runs instantly without requiring a GPU or Apple Metal environment.

---

## 6. Security and Performance Optimizations

* **Security (Host Isolation):** The Unix domain socket (`ipc:///tmp/vllm_vision.sock`) must be created with `0600` file permissions. Only the system user running the Go and Python processes may read/write to it, preventing unauthorized local prompt injection.
* **Go Memory Management:** Go passes byte slices (`[][]byte`) by reference. Allocation only occurs at the CGO boundary of the `zmq4` library, keeping Garbage Collection (GC) pressure exceptionally low.
* **Python Memory Limitation (Documented Risk):** While ZeroMQ prevents Base64 string expansion, Python's `PIL.Image.open(io.BytesIO(bytes))` inherently creates an in-memory copy of the byte array. When calculating the total system memory ceiling for high-concurrency workloads, allocate an extra $N \times \text{ImageSize}$ RAM buffer to account for PIL's processing.
* **Backpressure:** Setting the ZeroMQ High Water Mark (HWM) on both `DEALER` and `ROUTER` prevents memory exhaustion during sudden traffic spikes.

```

Would you like me to start writing the Go code for the `NewClient` constructor and the `DEALER` multiplexer `sync.Map` logic, or would you prefer to start with the Python backend implementation using `pytest`?

```
