package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	vllmipc "github.com/agenthands/zerovllm/client"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <image_path>")
	}
	imgPath := os.Args[1]
	if strings.HasPrefix(imgPath, "~/") {
		home, _ := os.UserHomeDir()
		imgPath = filepath.Join(home, imgPath[2:])
	}

	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		log.Fatalf("Failed to read image %s: %v", imgPath, err)
	}

	cfg := vllmipc.Config{
		SocketPath:        "ipc:///tmp/zerovllm.sock",
		RequestTimeout:    5 * time.Minute,
		ZMQHighWaterMark:  1000,
		MaxImages:         10,
		MaxImageSizeBytes: 50 * 1024 * 1024,
	}

	client, err := vllmipc.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Sending image to vision language model...")
	ctx := context.Background()
	start := time.Now()
	resp, err := client.GenerateVision(ctx, "Please describe exactly what is inside this image in detail.", [][]byte{imgData})
	if err != nil {
		log.Fatalf("GenerateVision failed: %v", err)
	}

	fmt.Printf("Time taken: %v\n", time.Since(start))
	fmt.Printf("Response: %s\n", resp.Text)
}
