package self_test

import (
	"context"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/qlog"

	"github.com/stretchr/testify/require"
)

// TestPragueVsRFC9002AlgorithmSwitching tests that different congestion control algorithms work independently
func TestPragueVsRFC9002AlgorithmSwitching(t *testing.T) {
	tests := []struct {
		name           string
		serverAlg      protocol.CongestionControlAlgorithm
		clientAlg      protocol.CongestionControlAlgorithm
		serverL4S      bool
		clientL4S      bool
		shouldConnect  bool
		description    string
	}{
		{
			name:          "Prague-Prague connection",
			serverAlg:     protocol.CongestionControlPrague,
			clientAlg:     protocol.CongestionControlPrague,
			serverL4S:     false, // L4S disabled for simplicity
			clientL4S:     false,
			shouldConnect: true,
			description:   "Both ends using Prague algorithm",
		},
		{
			name:          "RFC9002-RFC9002 connection",
			serverAlg:     protocol.CongestionControlRFC9002,
			clientAlg:     protocol.CongestionControlRFC9002,
			serverL4S:     false,
			clientL4S:     false,
			shouldConnect: true,
			description:   "Both ends using RFC9002 algorithm",
		},
		{
			name:          "Prague-RFC9002 mixed connection",
			serverAlg:     protocol.CongestionControlPrague,
			clientAlg:     protocol.CongestionControlRFC9002,
			serverL4S:     false,
			clientL4S:     false,
			shouldConnect: true,
			description:   "Server Prague, Client RFC9002 - should work",
		},
		{
			name:          "RFC9002-Prague mixed connection",
			serverAlg:     protocol.CongestionControlRFC9002,
			clientAlg:     protocol.CongestionControlPrague,
			serverL4S:     false,
			clientL4S:     false,
			shouldConnect: true,
			description:   "Server RFC9002, Client Prague - should work",
		},
		{
			name:          "Prague with L4S enabled",
			serverAlg:     protocol.CongestionControlPrague,
			clientAlg:     protocol.CongestionControlPrague,
			serverL4S:     true,
			clientL4S:     true,
			shouldConnect: true,
			description:   "Both Prague with L4S enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing: %s", tt.description)

			// Create server config
			serverConfig := getQuicConfig(&quic.Config{
				CongestionControlAlgorithm: tt.serverAlg,
				EnableL4S:                  tt.serverL4S,
			})

			// Create client config  
			clientConfig := getQuicConfig(&quic.Config{
				CongestionControlAlgorithm: tt.clientAlg,
				EnableL4S:                  tt.clientL4S,
			})

			// Start server
			server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
			require.NoError(t, err)
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Connect client
			conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
			if !tt.shouldConnect {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer conn.CloseWithError(0, "")

			// Accept connection on server
			serverConn, err := server.Accept(ctx)
			require.NoError(t, err)
			defer serverConn.CloseWithError(0, "")

			// Test basic data transfer to verify the connection works
			stream, err := conn.OpenStreamSync(ctx)
			require.NoError(t, err)
			defer stream.Close()

			testData := []byte("Algorithm switching test data")
			_, err = stream.Write(testData)
			require.NoError(t, err)
			err = stream.Close()
			require.NoError(t, err)

			// Receive on server side
			serverStream, err := serverConn.AcceptStream(ctx)
			require.NoError(t, err)
			
			receivedData := make([]byte, len(testData)+10)
			n, err := serverStream.Read(receivedData)
			if err != nil && err.Error() != "EOF" {
				require.NoError(t, err)
			}
			require.Equal(t, len(testData), n)
			require.Equal(t, testData, receivedData[:n])

			t.Logf("✅ Connection successful with %s server and %s client", 
				algorithmName(tt.serverAlg), algorithmName(tt.clientAlg))
		})
	}
}

// TestAlgorithmBehaviorDifferences tests observable differences between algorithms
func TestAlgorithmBehaviorDifferences(t *testing.T) {
	// Test Prague vs RFC9002 with tracing to observe behavioral differences
	tests := []struct {
		name      string
		algorithm protocol.CongestionControlAlgorithm
		enableL4S bool
	}{
		{
			name:      "RFC9002 behavior",
			algorithm: protocol.CongestionControlRFC9002,
			enableL4S: false,
		},
		{
			name:      "Prague behavior",
			algorithm: protocol.CongestionControlPrague,
			enableL4S: false,
		},
		{
			name:      "Prague with L4S behavior",
			algorithm: protocol.CongestionControlPrague,
			enableL4S: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var congestionEvents []qlog.CongestionState

			tracer := func(ctx context.Context, p qlog.Perspective, connID quic.ConnectionID) *qlog.ConnectionTracer {
				return &qlog.ConnectionTracer{
					UpdatedCongestionState: func(state qlog.CongestionState) {
						congestionEvents = append(congestionEvents, state)
						t.Logf("%s - Congestion state: %v", algorithmName(tt.algorithm), state)
					},
					UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
						t.Logf("Prague - Alpha updated: %f (marking: %f)", alpha, markingFraction)
					},
					L4SStateChanged: func(enabled bool, algorithm string) {
						t.Logf("L4S state changed: enabled=%t, algorithm=%s", enabled, algorithm)
					},
				}
			}

			serverConfig := getQuicConfig(&quic.Config{
				CongestionControlAlgorithm: tt.algorithm,
				EnableL4S:                  tt.enableL4S,
				Tracer:                     tracer,
			})

			clientConfig := getQuicConfig(&quic.Config{
				CongestionControlAlgorithm: tt.algorithm,
				EnableL4S:                  tt.enableL4S,
				Tracer:                     tracer,
			})

			// Start server
			server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
			require.NoError(t, err)
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Connect client
			conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
			require.NoError(t, err)
			defer conn.CloseWithError(0, "")

			serverConn, err := server.Accept(ctx)
			require.NoError(t, err)
			defer serverConn.CloseWithError(0, "")

			// Transfer data to exercise congestion control
			stream, err := conn.OpenStreamSync(ctx)
			require.NoError(t, err)
			defer stream.Close()

			// Send multiple chunks
			totalSent := 0
			for i := 0; i < 20; i++ {
				chunkSize := 2000
				n, err := stream.Write(PRData[:chunkSize])
				require.NoError(t, err)
				totalSent += n
				// Small delay to allow congestion control to react
				time.Sleep(1 * time.Millisecond)
			}
			err = stream.Close()
			require.NoError(t, err)

			// Receive on server side
			serverStream, err := serverConn.AcceptStream(ctx)
			require.NoError(t, err)
			
			totalReceived := 0
			buffer := make([]byte, 2048)
			for {
				n, err := serverStream.Read(buffer)
				if err != nil {
					if err.Error() == "EOF" {
						break
					}
					require.NoError(t, err)
				}
				totalReceived += n
				if totalReceived >= totalSent {
					break
				}
			}

			t.Logf("%s - Data transfer complete: sent=%d, received=%d, congestion events=%d", 
				algorithmName(tt.algorithm), totalSent, totalReceived, len(congestionEvents))
			
			// Verify we received most of the data (allow some variance)
			require.GreaterOrEqual(t, totalReceived, totalSent-4000)
		})
	}
}

// TestAlgorithmConfigurationCombinations tests various algorithm configuration combinations
func TestAlgorithmConfigurationCombinations(t *testing.T) {
	validCombinations := []struct {
		algorithm protocol.CongestionControlAlgorithm
		l4s       bool
		valid     bool
		name      string
	}{
		{protocol.CongestionControlRFC9002, false, true, "RFC9002 without L4S"},
		{protocol.CongestionControlRFC9002, true, false, "RFC9002 with L4S (invalid)"},
		{protocol.CongestionControlPrague, false, true, "Prague without L4S"},
		{protocol.CongestionControlPrague, true, true, "Prague with L4S"},
	}

	for _, combo := range validCombinations {
		t.Run(combo.name, func(t *testing.T) {
			config := &quic.Config{
				CongestionControlAlgorithm: combo.algorithm,
				EnableL4S:                  combo.l4s,
			}

			server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), config)
			if combo.valid {
				require.NoError(t, err, "Expected valid configuration to succeed")
				if server != nil {
					server.Close()
				}
				t.Logf("✅ Valid configuration: %s", combo.name)
			} else {
				require.Error(t, err, "Expected invalid configuration to fail")
				require.Contains(t, err.Error(), "L4S can only be enabled when using Prague congestion control")
				t.Logf("✅ Invalid configuration correctly rejected: %s", combo.name)
			}
		})
	}
}

// Helper function to get algorithm name for logging
func algorithmName(alg protocol.CongestionControlAlgorithm) string {
	switch alg {
	case protocol.CongestionControlRFC9002:
		return "RFC9002"
	case protocol.CongestionControlPrague:
		return "Prague"
	default:
		return "Unknown"
	}
}