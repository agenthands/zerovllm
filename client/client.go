package vllmipc

import (
	"context"
	"time"
)

// Config defines connection details and payload constraints.
type Config struct {
	SocketPath        string        // e.g., "ipc:///tmp/vllm.sock"
	MaxImages         int           // e.g., 6
	MaxImageSizeBytes int64         // e.g., 5 * 1024 * 1024 (5MB)
	RequestTimeout    time.Duration // Max wait time for LLM response
	ZMQHighWaterMark  int           // Backpressure threshold for DEALER socket (e.g., 1000)
}

// Client represents the thread-safe ZeroMQ client.
type Client interface {
	GenerateVision(ctx context.Context, prompt string, images [][]byte) (*VisionResponse, error)
	Close() error
}

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

// NewClient initializes the ZMQ context, applies the HWM, binds the DEALER socket,
// and spawns the background reader/writer multiplexer goroutines.
func NewClient(cfg Config) (Client, error) {
	return newMultiplexer(cfg)
}
