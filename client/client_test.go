package vllmipc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"github.com/google/uuid"
	zmq "github.com/pebbe/zmq4"
	"github.com/stretchr/testify/require"

	client "github.com/agenthands/zerovllm/client"
)

func mockPythonRouter(ctx context.Context, t *testing.T, socketPath string) {
	zctx, _ := zmq.NewContext()
	router, _ := zctx.NewSocket(zmq.ROUTER)
	defer router.Close()

	err := router.Bind(socketPath)
	require.NoError(t, err)

	poller := zmq.NewPoller()
	poller.Add(router, zmq.POLLIN)

	for {
		if ctx.Err() != nil {
			return
		}

		sockets, _ := poller.Poll(100 * time.Millisecond)
		for _, socket := range sockets {
			if socket.Socket == router {
				msg, _ := router.RecvMessageBytes(0)
				if len(msg) < 2 {
					continue
				}
				identity := msg[0]
				var meta client.VisionMetadata
				json.Unmarshal(msg[1], &meta)

				resp := client.VisionResponse{
					RequestID: meta.RequestID,
					Text:      fmt.Sprintf("Mock response to %s. Images: %d", meta.Prompt, len(msg)-2),
				}
				respBytes, _ := json.Marshal(resp)

				_, _ = router.SendMessage(identity, respBytes)
			}
		}
	}
}

func TestClientConfigConstraints(t *testing.T) {
	cfg := client.Config{
		SocketPath:        "ipc:///tmp/zerovllm_constraints_test.sock",
		MaxImages:         1,
		MaxImageSizeBytes: 100,
	}

	c, err := client.NewClient(cfg)
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()

	// 1. Too many images
	_, err = c.GenerateVision(ctx, "prompt", [][]byte{{1}, {2}})
	require.ErrorIs(t, err, client.ErrConstraintViolated)

	// 2. Images too large
	largeImage := make([]byte, 101)
	_, err = c.GenerateVision(ctx, "prompt", [][]byte{largeImage})
	require.ErrorIs(t, err, client.ErrConstraintViolated)
}

func TestClientConcurrentRouting(t *testing.T) {
	socketPath := fmt.Sprintf("ipc:///tmp/zerovllm_test_%s.sock", uuid.New().String()[:8])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mockPythonRouter(ctx, t, socketPath)
	time.Sleep(100 * time.Millisecond) // wait for bind

	cfg := client.Config{
		SocketPath:       socketPath,
		RequestTimeout:   2 * time.Second,
		ZMQHighWaterMark: 1000,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)
	defer c.Close()

	var wg sync.WaitGroup
	numRequests := 100
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()
			prompt := fmt.Sprintf("req-%d", id)
			img := []byte(fmt.Sprintf("img-%d", id))

			res, err := c.GenerateVision(context.Background(), prompt, [][]byte{img})
			assert.NoError(t, err)
			if res != nil {
				assert.Contains(t, res.Text, fmt.Sprintf("Mock response to %s.", prompt))
				assert.Contains(t, res.Text, "Images: 1")
			}
		}(i)
	}

	wg.Wait()
}
