package vllmipc_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	client "github.com/agenthands/zerovllm/client"
)

func createTestImage(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestE2EMultimodal(t *testing.T) {
	if os.Getenv("RUN_E2E") == "" {
		t.Skip("Skipping E2E VLLM test. Set RUN_E2E=1 to run.")
	}

	serverDir, err := filepath.Abs("../zeroserver")
	require.NoError(t, err)

	modelName := "mlx-community/Qwen2.5-VL-3B-Instruct-4bit"
	socketPath := fmt.Sprintf("ipc:///tmp/zerovllm_e2e_%s.sock", uuid.New().String()[:8])

	ctx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	// Spin up Python Server
	cmd := exec.CommandContext(ctx, "uv", "run", "python", "-m", "src.main", "--model", modelName, "--socket", socketPath)
	cmd.Dir = serverDir
	// Inherit stdout so we can see the VLLM loading progress
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = 2 * time.Second

	err = cmd.Start()
	require.NoError(t, err)

	// Since VLLM initialization is slow, we give requests a long timeout
	cfg := client.Config{
		SocketPath:        socketPath,
		MaxImages:         5,
		MaxImageSizeBytes: 10 * 1024 * 1024,
		RequestTimeout:    180 * time.Second, // Max 3 minutes for weights to load + inference
		ZMQHighWaterMark:  100,
	}

	// Give Python backend a moment to bind socket
	time.Sleep(1 * time.Second)

	c, err := client.NewClient(cfg)
	require.NoError(t, err)
	defer c.Close()

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 200*time.Second)
	defer reqCancel()

	img1 := createTestImage(color.RGBA{255, 0, 0, 255}) // Red square
	img2 := createTestImage(color.RGBA{0, 0, 255, 255}) // Blue square

	prompt := "I have provided two images. What is the dominant color of the first image, and what is the dominant color of the second image?"

	t.Log("Sending multimodal request to Python ZeroServer (vLLM)... this will wait until weights are loaded.")

	resp, err := c.GenerateVision(reqCtx, prompt, [][]byte{img1, img2})
	require.NoError(t, err, "failed to get generation response")

	t.Logf("Response ID: %s", resp.RequestID)
	t.Logf("Response Text: %s", resp.Text)

	require.NotEmpty(t, resp.Text)

	text := strings.ToLower(resp.Text)
	hasRed := strings.Contains(text, "red")
	hasBlue := strings.Contains(text, "blue")

	require.True(t, hasRed || hasBlue, "Response should have recognized at least one of the colors (red/blue).")
}
