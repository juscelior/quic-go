package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
)

const addr = "localhost:4242"

func main() {
	// Parse command line flags
	enableL4S := flag.Bool("enable-l4s", true, "Enable L4S (default: true)")
	flag.Parse()

	fmt.Println("[CLIENT] Starting L4S Performance Test Client")
	fmt.Println("[CLIENT] Connecting to", addr)

	// L4S configuration with Prague congestion control
	config := &quic.Config{
		EnableL4S:                  *enableL4S,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		MaxIdleTimeout:             300 * time.Second,
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	// Connect to server
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, config)
	if err != nil {
		log.Fatalf("[CLIENT] Failed to connect: %v", err)
	}
	defer conn.CloseWithError(0, "")

	fmt.Println("[CLIENT] Connected to server")

	// Start performance monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go monitorPerformance(ctx, conn)

	// Run performance test
	if err := runPerformanceTest(conn); err != nil {
		log.Fatalf("[CLIENT] Performance test failed: %v", err)
	}
}

func runPerformanceTest(conn *quic.Conn) error {
	// Test with different payload sizes
	testSizes := []int{1024, 10 * 1024, 100 * 1024, 1024 * 1024} // 1KB, 10KB, 100KB, 1MB

	for _, size := range testSizes {
		fmt.Printf("\n[CLIENT] Testing with %d bytes payload\n", size)

		if err := testEcho(conn, size); err != nil {
			return fmt.Errorf("echo test failed for size %d: %w", size, err)
		}

		// Brief pause between tests
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n[CLIENT] All performance tests completed successfully")
	return nil
}

func testEcho(conn *quic.Conn, payloadSize int) error {
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Generate test data
	data := make([]byte, payloadSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Send data and measure time
	start := time.Now()
	bytesWritten, err := stream.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Read echo response
	response := make([]byte, payloadSize)
	bytesRead, err := io.ReadFull(stream, response)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	duration := time.Since(start)

	// Verify data integrity
	if bytesWritten != bytesRead {
		return fmt.Errorf("data size mismatch: wrote %d, read %d", bytesWritten, bytesRead)
	}

	for i, b := range response {
		if b != data[i] {
			return fmt.Errorf("data corruption at byte %d: expected %d, got %d", i, data[i], b)
		}
	}

	throughput := float64(bytesWritten) / duration.Seconds() / (1024 * 1024) // MB/s

	fmt.Printf("[CLIENT] Echo test completed in %.3fs (%.2f MB/s)\n",
		duration.Seconds(), throughput)

	return nil
}

func monitorPerformance(ctx context.Context, conn *quic.Conn) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	fmt.Println("[PERF] Starting performance monitoring...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			// Get connection stats
			stats := conn.ConnectionStats()

			fmt.Printf("[PERF] RTT: %v | Bytes Sent: %d | Packets Sent: %d\n",
				stats.SmoothedRTT.Round(100*time.Microsecond),
				stats.BytesSent,
				stats.PacketsSent)
		}
	}
}
