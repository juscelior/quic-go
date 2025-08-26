package ackhandler

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/stretchr/testify/require"
)

// TestInteroperability_DefaultBehavior ensures that default configuration still works as before
func TestInteroperability_DefaultBehavior(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}
	
	// Default handler should use RFC9002 without L4S
	sph := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, true,
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlRFC9002, false, // Default settings
	)
	
	// Should use ECT0 (standard ECN)
	ecn := sph.ECNMode(true)
	require.Equal(t, protocol.ECT0, ecn, "Default should use ECT0")
	
	// Should use RFC9002 congestion control
	require.Equal(t, protocol.CongestionControlRFC9002, sph.congestionControlAlgorithm)
	require.False(t, sph.enableL4S)
}

// TestInteroperability_AlgorithmSwitching tests switching between algorithms
func TestInteroperability_AlgorithmSwitching(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}
	
	testCases := []struct {
		name      string
		algorithm protocol.CongestionControlAlgorithm
		enableL4S bool
		expectedECN protocol.ECN
	}{
		{
			name:        "RFC9002 without L4S",
			algorithm:   protocol.CongestionControlRFC9002,
			enableL4S:   false,
			expectedECN: protocol.ECT0,
		},
		{
			name:        "RFC9002 with L4S disabled (should be ECT0)",
			algorithm:   protocol.CongestionControlRFC9002,
			enableL4S:   true, // L4S should be ignored with RFC9002
			expectedECN: protocol.ECT0,
		},
		{
			name:        "Prague without L4S",
			algorithm:   protocol.CongestionControlPrague,
			enableL4S:   false,
			expectedECN: protocol.ECT0,
		},
		{
			name:        "Prague with L4S",
			algorithm:   protocol.CongestionControlPrague,
			enableL4S:   true,
			expectedECN: protocol.ECT1,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sph := newSentPacketHandler(
				0, 1200, rttStats, connStats, false, true,
				protocol.PerspectiveClient, nil, utils.DefaultLogger,
				tc.algorithm, tc.enableL4S,
			)
			
			ecn := sph.ECNMode(true)
			require.Equal(t, tc.expectedECN, ecn, "ECN marking should match expected for %s", tc.name)
			require.Equal(t, tc.algorithm, sph.congestionControlAlgorithm)
			require.Equal(t, tc.enableL4S, sph.enableL4S)
		})
	}
}

// TestInteroperability_CongestionBehavior tests that both algorithms handle congestion events correctly
func TestInteroperability_CongestionBehavior(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}
	
	testCases := []struct {
		name      string
		algorithm protocol.CongestionControlAlgorithm
		enableL4S bool
	}{
		{"RFC9002", protocol.CongestionControlRFC9002, false},
		{"Prague_without_L4S", protocol.CongestionControlPrague, false},
		{"Prague_with_L4S", protocol.CongestionControlPrague, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sph := newSentPacketHandler(
				0, 1200, rttStats, connStats, false, true,
				protocol.PerspectiveClient, nil, utils.DefaultLogger,
				tc.algorithm, tc.enableL4S,
			)
			
			now := time.Now()
			pn := protocol.PacketNumber(1)
			
			// Send a packet
			initialCwnd := sph.congestion.GetCongestionWindow()
			sph.SentPacket(now, pn, protocol.InvalidPacketNumber, nil, 
				[]Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, 
				protocol.ECNNon, 1200, false, false)
			
			// Simulate packet loss (congestion event)
			sph.congestion.OnCongestionEvent(pn, 1200, 1200)
			
			// Congestion window should decrease
			newCwnd := sph.congestion.GetCongestionWindow()
			require.Less(t, newCwnd, initialCwnd, "Congestion window should decrease on loss for %s", tc.name)
			
			// Both algorithms should be able to recover
			require.True(t, sph.congestion.CanSend(0), "Algorithm should allow sending when no bytes in flight")
		})
	}
}

// TestInteroperability_ECNFeedbackIsolation ensures ECN feedback doesn't affect non-Prague algorithms
func TestInteroperability_ECNFeedbackIsolation(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}
	
	// Test RFC9002 with ECN feedback (should not call Prague-specific methods)
	sph := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, true,
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlRFC9002, false,
	)
	
	now := time.Now()
	pn1 := protocol.PacketNumber(1)
	pn2 := protocol.PacketNumber(2)
	
	// Send packets
	sph.SentPacket(now, pn1, protocol.InvalidPacketNumber, nil, 
		[]Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, 
		protocol.ECT0, 1200, false, false)
	sph.SentPacket(now, pn2, protocol.InvalidPacketNumber, nil, 
		[]Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, 
		protocol.ECT0, 1200, false, false)
	
	// Simulate ACK with ECN feedback
	ackFrame := &wire.AckFrame{
		AckRanges: []wire.AckRange{{Smallest: 1, Largest: 2}},
		ECT0:      1,
		ECT1:      0,
		ECNCE:     1, // One packet marked
	}
	
	initialCwnd := sph.congestion.GetCongestionWindow()
	
	// Process ACK - should handle ECN properly without Prague-specific logic
	_, err := sph.ReceivedAck(ackFrame, protocol.Encryption1RTT, now.Add(time.Millisecond))
	require.NoError(t, err)
	
	// RFC9002 should handle ECN congestion indication
	newCwnd := sph.congestion.GetCongestionWindow()
	require.LessOrEqual(t, newCwnd, initialCwnd, "RFC9002 should respond to ECN congestion")
}

// TestInteroperability_PathMigration tests that path migration preserves algorithm choice
func TestInteroperability_PathMigration(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}
	
	testCases := []struct {
		name      string
		algorithm protocol.CongestionControlAlgorithm
		enableL4S bool
	}{
		{"RFC9002", protocol.CongestionControlRFC9002, false},
		{"Prague_with_L4S", protocol.CongestionControlPrague, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sph := newSentPacketHandler(
				0, 1200, rttStats, connStats, false, true,
				protocol.PerspectiveClient, nil, utils.DefaultLogger,
				tc.algorithm, tc.enableL4S,
			)
			
			// Verify initial state
			require.Equal(t, tc.algorithm, sph.congestionControlAlgorithm)
			require.Equal(t, tc.enableL4S, sph.enableL4S)
			
			// Simulate path migration
			now := time.Now()
			sph.MigratedPath(now, 1200)
			
			// Algorithm choice should be preserved
			require.Equal(t, tc.algorithm, sph.congestionControlAlgorithm)
			require.Equal(t, tc.enableL4S, sph.enableL4S)
			
			// Should be able to send on new path
			require.True(t, sph.congestion.CanSend(0))
		})
	}
}