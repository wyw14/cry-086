package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyw14/cry-086/internal/simulator"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:8080/api/v1/telemetry", "local telemetry endpoint")
	key := flag.String("key", "local-simulator-key", "simulator key")
	craneID := flag.String("crane", "crane-a", "demo crane identifier")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := &http.Client{Timeout: 3 * time.Second}
	scenario := simulator.Scenario{CraneID: *craneID, NodeID: "local-simulator", Start: time.Now().UTC(), Step: 200 * time.Millisecond, Samples: 12}
	for _, reading := range scenario.Readings() {
		if err := ctx.Err(); err != nil {
			return
		}
		payload, _ := json.Marshal(reading)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, *endpoint, bytes.NewReader(payload))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Simulator-Key", *key)
		response, err := client.Do(request)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = response.Body.Close()
		if response.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "telemetry rejected: %s\n", response.Status)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
