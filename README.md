Here is the repository description and the complete `README.md` for your project.
## zerovllm ⚡️


**zerovllm** is a highly optimized IPC bridge designed to connect Go microservices or web agents directly to a local Python `vLLM` instance using ZeroMQ. 

It is specifically engineered for **heavy multi-modal (vision) workloads** where standard OpenAI-compatible REST APIs become bottlenecks due to Base64 image encoding and JSON serialization CPU spikes.

## ⚠️ The Problem with REST + Vision Models
When sending multiple high-resolution images to a local LLM via standard HTTP/JSON:
1. **Base64 Bloat:** Encoding images inflates the payload size by ~33%.
2. **CPU Overhead:** JSON serialization/deserialization of massive Base64 strings blocks the Go runtime and spikes the Python GIL.
3. **Memory Thrashing:** Both Go and Python allocate multiple copies of the image string in RAM before it even reaches the GPU/Metal backend.

## 💡 The zerovllm Solution
`zerovllm` replaces the HTTP/JSON layer with a ZeroMQ `DEALER/ROUTER` socket architecture operating over Unix Domain Sockets (`ipc://`). 



* **Zero Base64:** Images are transmitted as raw binary frames.
* **Thread-Safe Concurrency:** Go multiplexes thousands of goroutines onto a single `DEALER` socket asynchronously.
* **Zero Network Latency:** Bypasses the localhost TCP stack entirely.
* **OpenTelemetry Native:** Seamlessly passes distributed tracing context across the Go/Python IPC boundary.

---

## 🏗 Architecture

* **Go Client (`vllmipc`):** Exposes a thread-safe `GenerateVision` method. It handles payload validation, assigns unique Request IDs, and routes responses back to the correct calling goroutine using a `sync.Map`.
* **Python Router (`vllm_router`):** A lightweight `zmq.asyncio` loop that receives multipart messages, loads raw bytes directly into `PIL.Image`, and schedules them on vLLM's `AsyncLLMEngine`.

---

## 🚀 Installation

### 1. Prerequisites
You need the ZeroMQ C library installed on your host system.
* **macOS:** `brew install zeromq pkg-config`
* **Ubuntu/Debian:** `sudo apt-get install libzmq3-dev pkg-config`

### 2. Go Client
```bash
go get [github.com/yourorg/zerovllm/client](https://github.com/yourorg/zerovllm/client)

```

### 3. Python Backend

We recommend using [uv](https://github.com/astral-sh/uv) or Poetry for dependency management.

```bash
cd server
uv venv
uv pip install -r pyproject.toml

```
---

## 💻 Quick Start

### 1. Start the Python vLLM Router
Run the backend server, specifying your multi-modal model (e.g., `microsoft/Phi-3.5-vision-instruct`).

```bash
python -m zerovllm.server.main \
  --model "microsoft/Phi-3.5-vision-instruct" \
  --socket "ipc:///tmp/zerovllm.sock"

```

### 2. Call from Go

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/agenthands/zerovllm/client"
)

func main() {
	// 1. Initialize the thread-safe ZeroMQ client
	cfg := client.Config{
		SocketPath:        "ipc:///tmp/zerovllm.sock",
		MaxImages:         6,
		MaxImageSizeBytes: 5 * 1024 * 1024, // 5MB limit
		RequestTimeout:    30 * time.Second,
		ZMQHighWaterMark:  1000,
	}
	
	vllmClient, err := client.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize zerovllm: %v", err)
	}
	defer vllmClient.Close()

	// 2. Load raw images (no Base64!)
	imgBytes, _ := os.ReadFile("screenshot.png")

	// 3. Execute inference concurrently
	ctx := context.Background()
	resp, err := vllmClient.GenerateVision(
		ctx, 
		"Describe the UI changes in these screenshots.", 
		[][]byte{imgBytes},
	)

	if err != nil {
		log.Fatalf("Inference failed: %v", err)
	}

	fmt.Printf("vLLM Response: %s\n", resp.Text)
}
```

---
## ⚙️ Configuration & Security

* **Permissions:** By default, the Python server creates the Unix domain socket (`/tmp/zerovllm.sock`) with `0600` permissions. Ensure your Go process runs under the same user, or adjust permissions if using separate Docker containers with a shared volume.
* **Memory Constraints:** If running on macOS (Apple Silicon), be mindful of shared Unified Memory. Adjust vLLM's `max_num_seqs` to prevent Metal Out-Of-Memory (OOM) errors during high-concurrency spikes. The ZeroMQ High Water Mark (HWM) will automatically drop requests if the queue exceeds safe limits.
---

## 🧪 Testing

**Go Client:**
```bash
cd client
go test -v -race ./...
```

**Python Server:**
```bash
cd server
pytest -v tests/=
```
## 🤝 Contributing
Contributions are welcome! Please ensure you adhere to the TDD methodology outlined in our spec and include tests for both the Go multiplexer and the Python router.


Would you like me to output the actual `client.go` and `multiplexer.go` implementation code so you can drop it directly into your new repository?

```
