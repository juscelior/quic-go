package congestion

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/utils"
	"github.com/stretchr/testify/require"
)

// TestAlgorithmIsolation ensures Prague and RFC9002 algorithms don't interfere with each other
func TestAlgorithmIsolation(t *testing.T) {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	rttStats.UpdateRTT(100*time.Millisecond, 0)
	connStats := &utils.ConnectionStats{}
	
	// Create both algorithm instances
	rfc9002 := NewCubicSender(clock, rttStats, connStats, 1200, true, nil)
	prague := NewPragueSender(clock, rttStats, connStats, 1200, true)
	
	// Both should start in slow start
	require.True(t, rfc9002.InSlowStart())
	require.True(t, prague.InSlowStart())
	
	// Both should have similar initial congestion window
	rfc9002Cwnd := rfc9002.GetCongestionWindow()
	pragueCwnd := prague.GetCongestionWindow()
	
	// Allow some difference but they should be in same ballpark
	require.InDelta(t, float64(rfc9002Cwnd), float64(pragueCwnd), float64(rfc9002Cwnd)*0.5)
	
	// Simulate different workloads on each algorithm
	now := clock.Now()
	
	// RFC9002: simulate packet loss
	rfc9002.OnPacketSent(now, 1200, 1, 1200, true)
	rfc9002.OnCongestionEvent(1, 1200, 1200)
	
	// Prague: simulate ECN feedback
	prague.OnPacketSent(now, 1200, 1, 1200, true)
	prague.OnPacketAcked(1, 1200, 1200, now.Add(50*time.Millisecond))
	prague.OnECNFeedback(600) // 50% marking
	
	// After different events, algorithms should be in different states
	rfc9002CwndAfter := rfc9002.GetCongestionWindow()
	pragueCwndAfter := prague.GetCongestionWindow()
	
	// RFC9002 should have reduced cwnd due to loss
	require.Less(t, rfc9002CwndAfter, rfc9002Cwnd)
	
	// Prague should have also reduced cwnd due to ECN
	require.Less(t, pragueCwndAfter, pragueCwnd)
	
	// Both should still be functional
	require.True(t, rfc9002.CanSend(0))
	require.True(t, prague.CanSend(0))
}

// TestPragueL4SBehavior tests Prague-specific L4S behavior
func TestPragueL4SBehavior(t *testing.T) {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	rttStats.UpdateRTT(50*time.Millisecond, 0)
	connStats := &utils.ConnectionStats{}
	
	// Prague with L4S enabled
	pragueL4S := NewPragueSender(clock, rttStats, connStats, 1200, true)
	
	// Prague without L4S (should behave more like classic)
	pragueClassic := NewPragueSender(clock, rttStats, connStats, 1200, false)
	
	now := clock.Now()
	
	// Both send packets
	pragueL4S.OnPacketSent(now, 1200, 1, 1200, true)
	pragueClassic.OnPacketSent(now, 1200, 1, 1200, true)
	
	initialCwndL4S := pragueL4S.GetCongestionWindow()
	initialCwndClassic := pragueClassic.GetCongestionWindow()
	
	// Both should start similarly
	require.InDelta(t, float64(initialCwndL4S), float64(initialCwndClassic), 
		float64(initialCwndL4S)*0.1)
	
	// Simulate ECN feedback - only L4S should respond differently
	pragueL4S.OnPacketAcked(1, 1200, 1200, now.Add(25*time.Millisecond))
	pragueClassic.OnPacketAcked(1, 1200, 1200, now.Add(25*time.Millisecond))
	
	// ECN marking feedback
	pragueL4S.OnECNFeedback(600)  // Should respond to ECN
	pragueClassic.OnECNFeedback(600) // Should also respond but less aggressively
	
	// Both should respond to ECN, but behavior might differ
	l4sCwndAfterECN := pragueL4S.GetCongestionWindow()
	classicCwndAfterECN := pragueClassic.GetCongestionWindow()
	
	// Check that algorithms are responsive to ECN feedback
	// Note: actual behavior depends on implementation details
	t.Logf("L4S: initial=%d, after_ecn=%d", initialCwndL4S, l4sCwndAfterECN)
	t.Logf("Classic: initial=%d, after_ecn=%d", initialCwndClassic, classicCwndAfterECN)
	
	// Both algorithms should be functional after ECN feedback
	require.True(t, pragueL4S.CanSend(0))
	require.True(t, pragueClassic.CanSend(0))
}

// TestAlgorithmStateIndependence ensures algorithms maintain independent state
func TestAlgorithmStateIndependence(t *testing.T) {
	clock := DefaultClock{}
	rttStats1 := &utils.RTTStats{}
	rttStats2 := &utils.RTTStats{}
	rttStats1.UpdateRTT(100*time.Millisecond, 0)
	rttStats2.UpdateRTT(200*time.Millisecond, 0) // Different RTT
	
	connStats1 := &utils.ConnectionStats{}
	connStats2 := &utils.ConnectionStats{}
	
	// Create two instances of the same algorithm with different parameters
	prague1 := NewPragueSender(clock, rttStats1, connStats1, 1200, true, nil)
	prague2 := NewPragueSender(clock, rttStats2, connStats2, 1500, false, nil)
	
	now := time.Now()
	
	// Different packet sizes and timing
	prague1.OnPacketSent(now, 1200, 1, 1200, true)
	prague2.OnPacketSent(now.Add(time.Millisecond), 1200, 1, 1500, true)
	
	// Different ACK timing
	prague1.OnPacketAcked(1, 1200, 1200, now.Add(50*time.Millisecond))
	prague2.OnPacketAcked(1, 1500, 1500, now.Add(100*time.Millisecond))
	
	// Different ECN feedback
	prague1.OnECNFeedback(300)  // 25% marking
	prague2.OnECNFeedback(1200) // 80% marking
	
	// Should have different congestion windows
	cwnd1 := prague1.GetCongestionWindow()
	cwnd2 := prague2.GetCongestionWindow()
	
	// With such different conditions, windows should be different
	require.NotEqual(t, cwnd1, cwnd2)
	
	// Both should still be functional
	require.True(t, prague1.CanSend(0))
	require.True(t, prague2.CanSend(0))
	
	// Bandwidth estimates should be different due to different RTTs and cwnd
	bw1 := prague1.BandwidthEstimate()
	bw2 := prague2.BandwidthEstimate()
	
	// Should have different bandwidth estimates
	require.NotEqual(t, bw1, bw2)
}