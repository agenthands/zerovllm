# zerovllm ⚡️

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
