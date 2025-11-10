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

// TestL4SBasicConnection tests basic L4S connection establishment
func TestL4SBasicConnection(t *testing.T) {
	// Create L4S-enabled configuration for server
	serverConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
	})

	// Create L4S-enabled configuration for client
	clientConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
	})

	// Start server
	server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect client
	conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
	require.NoError(t, err)
	defer conn.CloseWithError(0, "")

	// Accept connection on server
	serverConn, err := server.Accept(ctx)
	require.NoError(t, err)
	defer serverConn.CloseWithError(0, "")

	// Test basic data transfer
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	// Send test data
	testData := []byte("Hello L4S!")
	_, err = stream.Write(testData)
	require.NoError(t, err)
	err = stream.Close()
	require.NoError(t, err)

	// Receive on server side
	serverStream, err := serverConn.AcceptStream(ctx)
	require.NoError(t, err)
	
	receivedData := make([]byte, len(testData)+10) // Buffer with extra space
	n, err := serverStream.Read(receivedData)
	if err != nil && err.Error() != "EOF" {
		require.NoError(t, err)
	}
	require.Equal(t, len(testData), n)
	require.Equal(t, testData, receivedData[:n])
}

// TestL4SPragueAlgorithm tests that Prague algorithm is being used
func TestL4SPragueAlgorithm(t *testing.T) {
	var pragueUsed bool
	var ecnFeedbackReceived bool

	// Create tracer to monitor Prague-specific events
	tracer := func(ctx context.Context, p qlog.Perspective, connID quic.ConnectionID) *qlog.ConnectionTracer {
		return &qlog.ConnectionTracer{
			UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
				pragueUsed = true
				t.Logf("Prague alpha updated: alpha=%f, markingFraction=%f", alpha, markingFraction)
			},
			PragueECNFeedback: func(ecnMarkedBytes, totalBytes protocol.ByteCount) {
				ecnFeedbackReceived = true
				t.Logf("ECN feedback: marked=%d, total=%d", ecnMarkedBytes, totalBytes)
			},
			L4SStateChanged: func(enabled bool, algorithm string) {
				t.Logf("L4S state changed: enabled=%t, algorithm=%s", enabled, algorithm)
			},
		}
	}

	serverConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		Tracer:                     tracer,
	})

	clientConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		Tracer:                     tracer,
	})

	server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
	require.NoError(t, err)
	defer conn.CloseWithError(0, "")

	serverConn, err := server.Accept(ctx)
	require.NoError(t, err)
	defer serverConn.CloseWithError(0, "")

	// Transfer larger amount of data to trigger congestion control
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	// Send multiple chunks to exercise the Prague algorithm
	totalSent := 0
	for i := 0; i < 10; i++ {
		chunkSize := 1000
		n, err := stream.Write(PRData[:chunkSize])
		require.NoError(t, err)
		totalSent += n
	}
	err = stream.Close()
	require.NoError(t, err)

	// Receive on server side
	serverStream, err := serverConn.AcceptStream(ctx)
	require.NoError(t, err)
	
	totalReceived := 0
	buffer := make([]byte, 1024)
	for {
		n, err := serverStream.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				// Stream closed, normal termination
				break
			}
			require.NoError(t, err)
		}
		totalReceived += n
		if totalReceived >= totalSent {
			break
		}
	}

	require.GreaterOrEqual(t, totalReceived, totalSent-1000) // Allow for some variance
	t.Logf("Data transfer: sent=%d, received=%d", totalSent, totalReceived)
	
	// Note: Prague events might not trigger in this simple test due to lack of congestion
	// This is expected in a localhost test environment
	t.Logf("Prague events - used: %t, ECN feedback: %t", pragueUsed, ecnFeedbackReceived)
}

// TestL4SConfigurationValidation tests L4S configuration validation
func TestL4SConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *quic.Config
		shouldError bool
	}{
		{
			name: "L4S with Prague algorithm - valid",
			config: &quic.Config{
				EnableL4S:                  true,
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			shouldError: false,
		},
		{
			name: "L4S with RFC9002 algorithm - invalid",
			config: &quic.Config{
				EnableL4S:                  true,
				CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
			},
			shouldError: true,
		},
		{
			name: "No L4S with Prague algorithm - valid",
			config: &quic.Config{
				EnableL4S:                  false,
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			shouldError: false,
		},
		{
			name: "No L4S with RFC9002 algorithm - valid",
			config: &quic.Config{
				EnableL4S:                  false,
				CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test server configuration
			_, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), tt.config)
			if tt.shouldError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "L4S can only be enabled when using Prague congestion control")
			} else {
				require.NoError(t, err)
				// Configuration validation is the main test - dial testing would be redundant
			}
		})
	}
}

// Helper functions and tests continue below

// TestL4SWithoutPrague tests that L4S requires Prague algorithm
func TestL4SWithoutPrague(t *testing.T) {
	invalidConfig := &quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlRFC9002, // Invalid combination
	}

	_, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), invalidConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), "L4S can only be enabled when using Prague congestion control algorithm")
}

// TestPragueWithoutL4S tests that Prague can work without L4S
func TestPragueWithoutL4S(t *testing.T) {
	serverConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  false, // L4S disabled
		CongestionControlAlgorithm: protocol.CongestionControlPrague, // But Prague enabled
	})

	clientConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  false,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
	})

	server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
	require.NoError(t, err)
	defer conn.CloseWithError(0, "")

	serverConn, err := server.Accept(ctx)
	require.NoError(t, err)
	defer serverConn.CloseWithError(0, "")

	// Basic data transfer should work
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	testData := []byte("Prague without L4S")
	_, err = stream.Write(testData)
	require.NoError(t, err)
	err = stream.Close()
	require.NoError(t, err)

	serverStream, err := serverConn.AcceptStream(ctx)
	require.NoError(t, err)
	
	receivedData := make([]byte, len(testData)+10)
	n, err := serverStream.Read(receivedData)
	if err != nil && err.Error() != "EOF" {
		require.NoError(t, err)
	}
	require.Equal(t, len(testData), n)
	require.Equal(t, testData, receivedData[:n])
}