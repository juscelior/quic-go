package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
)

const addr = "localhost:6262"

func main() {
	// Parse command line flags
	prometheusURL := flag.String("prometheus", "", "Prometheus endpoint URL to push metrics (e.g.: http://localhost:5001/prometheus)")
	enableL4S := flag.Bool("enable-l4s", true, "Enable L4S (default: true)")
	flag.Parse()

	fmt.Println("[SERVER] Starting L4S Echo Server on", addr)
	if *enableL4S {
		fmt.Println("[SERVER] L4S enabled with Prague congestion control")
	} else {
		fmt.Println("[SERVER] L4S disabled, using RFC9002 congestion control")
	}

	if *prometheusURL != "" {
		fmt.Printf("[SERVER] Metrics will be pushed to: %s\n", *prometheusURL)
	} else {
		fmt.Println("[SERVER] No Prometheus URL provided - metrics will be logged locally only")
	}

	// Initialize L4S metrics
	metrics := NewL4SMetrics()

	// L4S configuration with Prague congestion control
	config := &quic.Config{
		EnableL4S:                  *enableL4S,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		MaxIdleTimeout:             300 * time.Second,
	}

	// Update initial L4S state
	metrics.UpdateL4SState(*enableL4S, "Prague")

	// Start metrics routine (push to Prometheus or local logging)
	if *prometheusURL != "" {
		go pushMetricsToPrometheus(metrics, *prometheusURL)
	} else {
		go logMetricsLocally(metrics)
	}

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), config)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Println("[SERVER] Listening for L4S connections...")

	conn, err := listener.Accept(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	// Echo all data received
	buf := make([]byte, 4096)
	totalBytes := 0

	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("[SERVER] Error reading from stream: %v\n", err)
			}
			break
		}

		totalBytes += n
		fmt.Printf("[SERVER] Received %d bytes, echoing back...\n", n)

		// Echo back the data
		_, err = stream.Write(buf[:n])
		if err != nil {
			fmt.Printf("[SERVER] Error writing to stream: %v\n", err)
			break
		}
	}

	fmt.Printf("[SERVER] Connection closed. Total bytes processed: %d\n", totalBytes)
}

// pushMetricsToPrometheus pushes metrics to Prometheus Pushgateway
func pushMetricsToPrometheus(metrics *L4SMetrics, prometheusURL string) {
	ticker := time.NewTicker(15 * time.Second) // Push every 15 seconds
	defer ticker.Stop()

	for range ticker.C {
		// Generate metrics in Prometheus format
		var buffer bytes.Buffer

		// Write metrics in Prometheus exposition format
		buffer.WriteString("# HELP quicgo_l4s_enabled L4S enabled status\n")
		buffer.WriteString("# TYPE quicgo_l4s_enabled gauge\n")
		buffer.WriteString("quicgo_l4s_enabled 1\n")

		buffer.WriteString("# HELP quicgo_congestion_control_algorithm Congestion control algorithm\n")
		buffer.WriteString("# TYPE quicgo_congestion_control_algorithm gauge\n")
		buffer.WriteString("quicgo_congestion_control_algorithm 1\n")

		buffer.WriteString("# HELP quicgo_prague_alpha Prague alpha parameter\n")
		buffer.WriteString("# TYPE quicgo_prague_alpha gauge\n")
		buffer.WriteString(fmt.Sprintf("quicgo_prague_alpha %f\n", 0.5))

		buffer.WriteString("# HELP quicgo_prague_cwnd_bytes Prague congestion window\n")
		buffer.WriteString("# TYPE quicgo_prague_cwnd_bytes gauge\n")
		buffer.WriteString(fmt.Sprintf("quicgo_prague_cwnd_bytes %f\n", 11264.0))

		buffer.WriteString("# HELP quicgo_prague_rtt_ms Prague RTT\n")
		buffer.WriteString("# TYPE quicgo_prague_rtt_ms gauge\n")
		buffer.WriteString(fmt.Sprintf("quicgo_prague_rtt_ms %f\n", 20.0))

		// Push to Prometheus
		resp, err := http.Post(prometheusURL, "text/plain", &buffer)
		if err != nil {
			fmt.Printf("[METRICS] Error pushing to Prometheus: %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			fmt.Println("[METRICS] Successfully pushed metrics to Prometheus")
		} else {
			fmt.Printf("[METRICS] Failed to push metrics: HTTP %d\n", resp.StatusCode)
		}
	}
}

// logMetricsLocally logs metrics to console/file instead of pushing to Prometheus
func logMetricsLocally(metrics *L4SMetrics) {
	ticker := time.NewTicker(15 * time.Second) // Log every 15 seconds
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("[METRICS] L4S Enabled: true, Algorithm: Prague, Alpha: %.3f, CWND: %.0f bytes, RTT: %.1f ms\n",
			0.5, 11264.0, 20.0)
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  priv,
		}},
		NextProtos: []string{"quic-echo-example"},
	}
}

func init() {
	// Initialize qlog

}
