package vllmipc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	zmq "github.com/pebbe/zmq4"
)

type multiplexer struct {
	cfg      Config
	dealer   *zmq.Socket
	sendChan chan [][]byte
	requests sync.Map // string (RequestID) -> chan *VisionResponse
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newMultiplexer(cfg Config) (Client, error) {
	zctx, err := zmq.NewContext()
	if err != nil {
		return nil, fmt.Errorf("failed to create zmq context: %w", err)
	}

	dealer, err := zctx.NewSocket(zmq.DEALER)
	if err != nil {
		return nil, fmt.Errorf("failed to create DEALER socket: %w", err)
	}

	if cfg.ZMQHighWaterMark > 0 {
		dealer.SetSndhwm(cfg.ZMQHighWaterMark)
		dealer.SetRcvhwm(cfg.ZMQHighWaterMark)
	}

	if err := dealer.Connect(cfg.SocketPath); err != nil {
		return nil, fmt.Errorf("failed to connect DEALER to %s: %w", cfg.SocketPath, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	hwm := cfg.ZMQHighWaterMark
	if hwm <= 0 {
		hwm = 1000
	}

	m := &multiplexer{
		cfg:      cfg,
		dealer:   dealer,
		sendChan: make(chan [][]byte, hwm),
		cancel:   cancel,
	}

	m.wg.Add(1)
	go m.reactorLoop(ctx)

	return m, nil
}

func (m *multiplexer) reactorLoop(ctx context.Context) {
	defer m.wg.Done()

	poller := zmq.NewPoller()
	poller.Add(m.dealer, zmq.POLLIN)

	for {
		if ctx.Err() != nil {
			return
		}

		// Drain the send channel
	drainSend:
		for i := 0; i < 1000; i++ {
			select {
			case parts := <-m.sendChan:
				args := make([]interface{}, len(parts))
				for j, v := range parts {
					args[j] = v
				}
				m.dealer.SendMessage(args...)
			default:
				break drainSend
			}
		}

		// Poll incoming
		sockets, err := poller.Poll(50 * time.Millisecond)
		if err != nil {
			continue
		}

		for _, socket := range sockets {
			if socket.Socket == m.dealer {
				// Drain incoming sockets
				for {
					parts, err := m.dealer.RecvMessageBytes(zmq.DONTWAIT)
					if err != nil {
						break // EAGAIN
					}
					if len(parts) >= 1 {
						var resp VisionResponse
						if err := json.Unmarshal(parts[0], &resp); err == nil {
							if ch, ok := m.requests.LoadAndDelete(resp.RequestID); ok {
								resChan := ch.(chan *VisionResponse)
								resChan <- &resp
								close(resChan)
							}
						}
					}
				}
			}
		}
	}
}

// GenerateVision sends a payload to vLLM multiplexer and waits for the specific response.
func (m *multiplexer) GenerateVision(ctx context.Context, prompt string, images [][]byte) (*VisionResponse, error) {
	if m.cfg.MaxImages > 0 && len(images) > m.cfg.MaxImages {
		return nil, fmt.Errorf("%w: too many images (got %d, max %d)", ErrConstraintViolated, len(images), m.cfg.MaxImages)
	}

	var totalSize int64
	for _, img := range images {
		totalSize += int64(len(img))
	}
	if m.cfg.MaxImageSizeBytes > 0 && totalSize > m.cfg.MaxImageSizeBytes {
		return nil, fmt.Errorf("%w: images too large (got %d bytes, max %d)", ErrConstraintViolated, totalSize, m.cfg.MaxImageSizeBytes)
	}

	reqID := uuid.New().String()
	resChan := make(chan *VisionResponse, 1)

	m.requests.Store(reqID, resChan)

	metadata := VisionMetadata{
		RequestID:    reqID,
		Prompt:       prompt,
		TraceContext: ExtractTraceContext(ctx),
	}

	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		m.requests.Delete(reqID)
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	msg := make([][]byte, 0, 1+len(images))
	msg = append(msg, metaBytes)
	msg = append(msg, images...)

	select {
	case m.sendChan <- msg:
	default:
		m.requests.Delete(reqID)
		return nil, fmt.Errorf("%w: send queue full", ErrEngineTimeout)
	}

	timeoutCtx := ctx
	var cancel context.CancelFunc
	if m.cfg.RequestTimeout > 0 {
		timeoutCtx, cancel = context.WithTimeout(ctx, m.cfg.RequestTimeout)
		defer cancel()
	}

	select {
	case res := <-resChan:
		if res.Error != "" {
			return res, fmt.Errorf("%w: %s", ErrInvalidResponse, res.Error)
		}
		return res, nil
	case <-timeoutCtx.Done():
		m.requests.Delete(reqID)
		return nil, ErrEngineTimeout
	}
}

func (m *multiplexer) Close() error {
	m.cancel()
	m.wg.Wait()
	m.dealer.SetLinger(0)
	m.dealer.Close()
	return nil
}
