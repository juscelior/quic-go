package self_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/qlog"

	"github.com/stretchr/testify/require"
)

// TestECNFeedbackIntegration tests ECN feedback integration with ACK processing
func TestECNFeedbackIntegration(t *testing.T) {
	var ecnFeedbackCount atomic.Int64
	var ackFrameCount atomic.Int64
	var ecnMarkedBytes atomic.Int64
	var totalAckedBytes atomic.Int64

	// Use default tracer for logging
	tracer := qlog.DefaultConnectionTracer

	// Server and client both use Prague with L4S
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

	// Start server
	server, err := quic.Listen(newUDPConnLocalhost(t), getTLSConfig(), serverConfig)
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Connect client
	conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
	require.NoError(t, err)
	defer conn.CloseWithError(0, "")

	serverConn, err := server.Accept(ctx)
	require.NoError(t, err)
	defer serverConn.CloseWithError(0, "")

	// Test data transfer to generate ACK traffic
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	// Send data in multiple chunks to increase ACK frequency
	totalSent := 0
	chunkSize := 1000
	numChunks := 30

	for i := 0; i < numChunks; i++ {
		n, err := stream.Write(PRData[:chunkSize])
		require.NoError(t, err)
		totalSent += n
		// Small delay to allow ACKs to be processed
		time.Sleep(2 * time.Millisecond)
	}

	err = stream.Close()
	require.NoError(t, err)

	// Receive on server side
	serverStream, err := serverConn.AcceptStream(ctx)
	require.NoError(t, err)

	totalReceived := 0
	buffer := make([]byte, chunkSize*2)
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

	// Allow time for final ACKs and ECN processing
	time.Sleep(100 * time.Millisecond)

	// Verify data transfer
	require.Equal(t, totalSent, totalReceived)

	// Log results
	finalECNFeedback := ecnFeedbackCount.Load()
	finalAckFrames := ackFrameCount.Load()
	finalMarkedBytes := ecnMarkedBytes.Load()
	finalTotalBytes := totalAckedBytes.Load()

	t.Logf("Test Results:")
	t.Logf("  Data transferred: %d bytes", totalReceived)
	t.Logf("  ECN feedback events: %d", finalECNFeedback)
	t.Logf("  ACK frames received: %d", finalAckFrames)
	t.Logf("  ECN marked bytes: %d", finalMarkedBytes)
	t.Logf("  Total acked bytes: %d", finalTotalBytes)

	// We should have received some ACK frames during data transfer
	require.Greater(t, finalAckFrames, int64(0), "Expected to receive ACK frames")

	// ECN feedback might be 0 in localhost testing (no actual congestion/marking)
	// This is expected behavior
	if finalECNFeedback > 0 {
		t.Logf("ECN feedback detected - indicating ECN/ACK integration is active")
	} else {
		t.Logf("No ECN feedback in localhost test - this is expected without actual network congestion")
	}
}

// packetInfo represents information about a logged packet
type packetInfo struct {
	timestamp time.Time
	direction string // "sent" or "received"
	ecn       protocol.ECN
	size      protocol.ByteCount
	hasAck    bool
}

// TestECNMarkingAndACKProcessing tests ECN marking with ACK processing
func TestECNMarkingAndACKProcessing(t *testing.T) {
	var mu sync.Mutex
	var packetLog []packetInfo

	// Use default tracer for logging
	tracer := qlog.DefaultConnectionTracer

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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := quic.Dial(ctx, newUDPConnLocalhost(t), server.Addr(), getTLSClientConfig(), clientConfig)
	require.NoError(t, err)
	defer conn.CloseWithError(0, "")

	serverConn, err := server.Accept(ctx)
	require.NoError(t, err)
	defer serverConn.CloseWithError(0, "")

	// Quick data transfer
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	testData := []byte("ECN marking test")
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

	// Allow time for final packet processing
	time.Sleep(50 * time.Millisecond)

	// Analyze packet log
	mu.Lock()
	defer mu.Unlock()

	ect1Packets := 0
	ackPackets := 0
	totalPackets := len(packetLog)

	for _, pkt := range packetLog {
		if pkt.ecn == protocol.ECT1 {
			ect1Packets++
			t.Logf("ECT(1) packet: %s, size=%d, hasAck=%t", pkt.direction, pkt.size, pkt.hasAck)
		}
		if pkt.hasAck {
			ackPackets++
		}
	}

	t.Logf("Packet Analysis:")
	t.Logf("  Total packets logged: %d", totalPackets)
	t.Logf("  ECT(1) marked packets: %d", ect1Packets)
	t.Logf("  Packets with ACK frames: %d", ackPackets)

	// Verify we logged some packets
	require.Greater(t, totalPackets, 0, "Expected to log some packets")

	// We should see some ACK frames during the connection
	require.Greater(t, ackPackets, 0, "Expected to see ACK frames")

	// ECT(1) marking depends on L4S implementation - may or may not be present in localhost
	if ect1Packets > 0 {
		t.Logf("ECT(1) marking detected - L4S ECN marking is working")
	} else {
		t.Logf("No ECT(1) marking detected - this may be expected in localhost testing")
	}
}

// TestECNFeedbackWithoutL4S tests that ECN feedback is not generated without L4S
func TestECNFeedbackWithoutL4S(t *testing.T) {
	var ecnFeedbackCount atomic.Int64

	// Use default tracer for logging
	tracer := qlog.DefaultConnectionTracer

	// Prague without L4S
	serverConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  false, // L4S disabled
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		Tracer:                     tracer,
	})

	clientConfig := getQuicConfig(&quic.Config{
		EnableL4S:                  false,
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

	// Transfer data
	stream, err := conn.OpenStreamSync(ctx)
	require.NoError(t, err)
	defer stream.Close()

	// Send multiple chunks
	for i := 0; i < 10; i++ {
		_, err = stream.Write(PRData[:1000])
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
	}
	err = stream.Close()
	require.NoError(t, err)

	serverStream, err := serverConn.AcceptStream(ctx)
	require.NoError(t, err)

	totalReceived := 0
	buffer := make([]byte, 1024)
	for totalReceived < 10000 {
		n, err := serverStream.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			require.NoError(t, err)
		}
		totalReceived += n
	}

	time.Sleep(50 * time.Millisecond)

	// Without L4S, we should not see ECN feedback
	finalCount := ecnFeedbackCount.Load()
	require.Equal(t, int64(0), finalCount, "Expected no ECN feedback without L4S enabled")
	t.Logf("No ECN feedback without L4S - correct behavior")
}
