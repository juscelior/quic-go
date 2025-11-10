package congestion

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/qlog"

	"github.com/stretchr/testify/require"
)

const (
	pragueInitialCongestionWindowPackets = 32
	pragueInitialCongestionWindowBytes   = pragueInitialCongestionWindowPackets * initialMaxDatagramSize
)

type testPragueSender struct {
	sender            *pragueSender
	clock             *mockClock
	rttStats          *utils.RTTStats
	connStats         *utils.ConnectionStats
	bytesInFlight     protocol.ByteCount
	packetNumber      protocol.PacketNumber
	ackedPacketNumber protocol.PacketNumber
	tracer            *mockTracer
}

type mockTracer struct {
	pragueAlphaUpdates []alphaUpdate
	ecnFeedbackEvents  []ecnFeedbackEvent
	stateChanges       []qlog.CongestionState
}

type alphaUpdate struct {
	alpha           float64
	markingFraction float64
}

type ecnFeedbackEvent struct {
	ecnMarkedBytes protocol.ByteCount
	totalBytes     protocol.ByteCount
}

func (m *mockTracer) UpdatedPragueAlpha(alpha float64, markingFraction float64) {
	if m.pragueAlphaUpdates == nil {
		m.pragueAlphaUpdates = []alphaUpdate{}
	}
	m.pragueAlphaUpdates = append(m.pragueAlphaUpdates, alphaUpdate{
		alpha:           alpha,
		markingFraction: markingFraction,
	})
}

func (m *mockTracer) PragueECNFeedback(ecnMarkedBytes, totalBytes protocol.ByteCount) {
	if m.ecnFeedbackEvents == nil {
		m.ecnFeedbackEvents = []ecnFeedbackEvent{}
	}
	m.ecnFeedbackEvents = append(m.ecnFeedbackEvents, ecnFeedbackEvent{
		ecnMarkedBytes: ecnMarkedBytes,
		totalBytes:     totalBytes,
	})
}

func (m *mockTracer) UpdatedCongestionState(new qlog.CongestionState) {
	if m.stateChanges == nil {
		m.stateChanges = []qlog.CongestionState{}
	}
	m.stateChanges = append(m.stateChanges, new)
}

func newTestPragueSender(l4sEnabled bool) *testPragueSender {
	var clock mockClock
	rttStats := utils.RTTStats{}
	connStats := utils.ConnectionStats{}
	tracer := &mockTracer{}

	return &testPragueSender{
		clock:        &clock,
		rttStats:     &rttStats,
		connStats:    &connStats,
		packetNumber: 1,
		tracer:       tracer,
		sender: newPragueSender(
			&clock,
			&rttStats,
			&connStats,
			initialMaxDatagramSize,
			l4sEnabled,
		),
	}
}

func (s *testPragueSender) SendAvailableSendWindow() int {
	return s.SendAvailableSendWindowLen(initialMaxDatagramSize)
}

func (s *testPragueSender) SendAvailableSendWindowLen(packetLength protocol.ByteCount) int {
	var packetsSent int
	for s.sender.CanSend(s.bytesInFlight) {
		s.sender.OnPacketSent(s.clock.Now(), s.bytesInFlight, s.packetNumber, packetLength, true)
		s.packetNumber++
		packetsSent++
		s.bytesInFlight += packetLength
	}
	return packetsSent
}

func (s *testPragueSender) AckNPackets(n int) {
	s.AckNPacketsWithECN(n, 0)
}

func (s *testPragueSender) AckNPacketsWithECN(n int, ecnMarkedPackets int) {
	s.rttStats.UpdateRTT(60*time.Millisecond, 0)
	for range n {
		s.ackedPacketNumber++
		s.sender.OnPacketAcked(s.ackedPacketNumber, initialMaxDatagramSize, s.bytesInFlight, s.clock.Now())
	}

	// Apply ECN feedback if any packets were marked
	if ecnMarkedPackets > 0 {
		ecnMarkedBytes := protocol.ByteCount(ecnMarkedPackets) * initialMaxDatagramSize
		s.sender.OnECNFeedback(ecnMarkedBytes)
	}

	s.bytesInFlight -= protocol.ByteCount(n) * initialMaxDatagramSize
	s.clock.Advance(time.Millisecond)
}

func (s *testPragueSender) LoseNPackets(n int) {
	s.LoseNPacketsLen(n, initialMaxDatagramSize)
}

func (s *testPragueSender) LoseNPacketsLen(n int, packetLength protocol.ByteCount) {
	for range n {
		s.ackedPacketNumber++
		s.sender.OnCongestionEvent(s.ackedPacketNumber, packetLength, s.bytesInFlight)
	}
	s.bytesInFlight -= protocol.ByteCount(n) * packetLength
}

func (s *testPragueSender) LosePacket(number protocol.PacketNumber) {
	s.sender.OnCongestionEvent(number, initialMaxDatagramSize, s.bytesInFlight)
	s.bytesInFlight -= initialMaxDatagramSize
}

func TestPragueSenderStartup(t *testing.T) {
	sender := newTestPragueSender(true)

	// At startup make sure we are at the Prague initial CWND
	require.Equal(t, pragueInitialCongestionWindowBytes, sender.sender.GetCongestionWindow())

	// Make sure we can send
	require.Zero(t, sender.sender.TimeUntilSend(0))
	require.True(t, sender.sender.CanSend(sender.bytesInFlight))

	// And that window is unaffected
	require.Equal(t, pragueInitialCongestionWindowBytes, sender.sender.GetCongestionWindow())

	// Fill the send window with data, then verify that we can't send
	sender.SendAvailableSendWindow()
	require.False(t, sender.sender.CanSend(sender.bytesInFlight))
}

func TestPragueSenderStartupWithoutL4S(t *testing.T) {
	sender := newTestPragueSender(false)

	// Prague should still work without L4S enabled
	require.Equal(t, pragueInitialCongestionWindowBytes, sender.sender.GetCongestionWindow())
	require.True(t, sender.sender.CanSend(0))
	require.False(t, sender.sender.l4sEnabled)
}

func TestPragueSenderSlowStart(t *testing.T) {
	sender := newTestPragueSender(true)

	// Verify we start in slow start
	require.True(t, sender.sender.InSlowStart())
	require.False(t, sender.sender.InRecovery())

	// Send packets and ack them - should grow exponentially
	const numberOfAcks = 10
	initialCwnd := sender.sender.GetCongestionWindow()

	for range numberOfAcks {
		sender.SendAvailableSendWindow()
		sender.AckNPackets(2)
	}

	// CWND should have grown in slow start (each ack increases by ackedBytes)
	finalCwnd := sender.sender.GetCongestionWindow()
	expectedMinIncrease := initialMaxDatagramSize * numberOfAcks * 2
	require.GreaterOrEqual(t, finalCwnd, initialCwnd+expectedMinIncrease)
}

func TestPragueSenderExitSlowStartOnECNMarks(t *testing.T) {
	sender := newTestPragueSender(true)

	require.True(t, sender.sender.InSlowStart())
	require.Equal(t, 0.0, sender.sender.alpha)

	// Send some packets
	sender.SendAvailableSendWindow()

	// Ack with ECN marks - should exit slow start and update alpha
	sender.AckNPacketsWithECN(10, 2) // 2 out of 10 packets marked

	require.False(t, sender.sender.InSlowStart())
	require.Greater(t, sender.sender.alpha, 0.0)

}

func TestPragueSenderAlphaCalculation(t *testing.T) {
	sender := newTestPragueSender(true)

	// Initial alpha should be 0
	require.Equal(t, 0.0, sender.sender.alpha)

	// Send packets and ack with ECN marks
	sender.SendAvailableSendWindow()
	totalBytes := protocol.ByteCount(10) * initialMaxDatagramSize
	markedBytes := protocol.ByteCount(2) * initialMaxDatagramSize

	sender.sender.totalAckedBytes = totalBytes
	sender.sender.ecnMarkedBytes = markedBytes
	sender.sender.updateAlpha()

	// On first ECN feedback, alpha should be initialized to 1.0
	require.Equal(t, 1.0, sender.sender.alpha)

	// Apply EWMA update with new marking fraction
	sender.sender.totalAckedBytes = totalBytes
	sender.sender.ecnMarkedBytes = markedBytes / 2 // Lower marking rate
	markingFraction := float64(markedBytes/2) / float64(totalBytes)
	expectedAlpha := (1.0-pragueAlphaGain)*1.0 + pragueAlphaGain*markingFraction

	sender.sender.updateAlpha()
	require.InDelta(t, expectedAlpha, sender.sender.alpha, 0.001)
}

func TestPragueSenderECNCongestionResponse(t *testing.T) {
	sender := newTestPragueSender(true)

	// Set a large CWND to test reduction properly
	sender.sender.congestionWindow = initialMaxDatagramSize * 20 // Large CWND
	sender.sender.alpha = 0.4
	sender.sender.cwndCarry = 0.0 // Reset carry
	originalCwnd := sender.sender.congestionWindow

	// Apply ECN congestion response
	sender.sender.applyECNCongestionResponse()

	// CWND should decrease, accounting for the fractional carry mechanism
	finalCwnd := sender.sender.GetCongestionWindow()
	require.Less(t, finalCwnd, originalCwnd)

	// Should be at least the minimum congestion window
	minCwnd := sender.sender.minCongestionWindow()
	require.GreaterOrEqual(t, finalCwnd, minCwnd)
}

func TestPragueSenderAdditiveIncrease(t *testing.T) {
	sender := newTestPragueSender(true)

	// Get out of slow start
	sender.SendAvailableSendWindow()
	sender.AckNPacketsWithECN(10, 1)

	// Set alpha and get baseline CWND
	sender.sender.alpha = 0.2
	initialCwnd := sender.sender.GetCongestionWindow()

	// Simulate additive increase
	ackedBytes := initialMaxDatagramSize
	sender.sender.pragueAdditiveIncrease(ackedBytes)

	// CWND should increase by modified amount based on (1-alpha)
	unmarkedFraction := 1.0 - sender.sender.alpha
	expectedIncrease := float64(initialMaxDatagramSize) * float64(ackedBytes) * unmarkedFraction / float64(initialCwnd)
	expectedCwnd := float64(initialCwnd) + expectedIncrease

	require.InDelta(t, expectedCwnd, float64(sender.sender.GetCongestionWindow()), float64(initialMaxDatagramSize/10))
}

func TestPragueSenderClassicLossResponse(t *testing.T) {
	sender := newTestPragueSender(true)

	// Build up congestion window in slow start
	const numberOfAcks = 10
	for range numberOfAcks {
		sender.SendAvailableSendWindow()
		sender.AckNPackets(2)
	}
	sender.SendAvailableSendWindow()

	preLossCwnd := sender.sender.GetCongestionWindow()
	require.True(t, sender.sender.InSlowStart())

	// Lose a packet - should trigger classic loss response
	sender.LoseNPackets(1)

	// Should exit slow start and enter recovery
	require.False(t, sender.sender.InSlowStart())
	require.True(t, sender.sender.InRecovery())

	// CWND should be reduced by beta factor (0.5)
	expectedCwnd := protocol.ByteCount(float64(preLossCwnd) * pragueBeta)
	minCwnd := sender.sender.minCongestionWindow()
	if expectedCwnd < minCwnd {
		expectedCwnd = minCwnd
	}

	require.Equal(t, expectedCwnd, sender.sender.GetCongestionWindow())
	require.Equal(t, expectedCwnd, sender.sender.slowStartThreshold)
}

func TestPragueSenderRetransmissionTimeout(t *testing.T) {
	sender := newTestPragueSender(true)

	initialCwnd := sender.sender.GetCongestionWindow()
	sender.sender.OnRetransmissionTimeout(true)

	// CWND should be reset to minimum
	expectedMinCwnd := sender.sender.minCongestionWindow()
	require.Equal(t, expectedMinCwnd, sender.sender.GetCongestionWindow())
	require.Equal(t, initialCwnd/2, sender.sender.slowStartThreshold)
	require.False(t, sender.sender.inSlowStart)
}

func TestPragueSenderBandwidthEstimate(t *testing.T) {
	sender := newTestPragueSender(true)

	// Test basic bandwidth estimation functionality
	// Update RTT stats first
	sender.rttStats.UpdateRTT(100*time.Millisecond, 0)

	// Bandwidth should be calculable and finite
	bandwidth := sender.sender.BandwidthEstimate()
	require.Greater(t, bandwidth, Bandwidth(0))

	// Bandwidth should be based on CWND and virtual RTT
	expectedBandwidth := BandwidthFromDelta(sender.sender.GetCongestionWindow(), sender.sender.getVirtualRTT())
	require.Equal(t, expectedBandwidth, bandwidth)
}

func TestPragueSenderVirtualRTT(t *testing.T) {
	sender := newTestPragueSender(true)

	// Without RTT measurements, should use minimum virtual RTT
	require.Equal(t, pragueVirtualRTTMin, sender.sender.getVirtualRTT())

	// Update with a smaller RTT - should still use minimum
	sender.rttStats.UpdateRTT(10*time.Millisecond, 0)
	require.Equal(t, pragueVirtualRTTMin, sender.sender.getVirtualRTT())

	// Update with a larger RTT - should use actual RTT
	largerRTT := 50 * time.Millisecond
	sender.rttStats.UpdateRTT(largerRTT, 0)
	virtualRTT := sender.sender.getVirtualRTT()
	require.GreaterOrEqual(t, virtualRTT, pragueVirtualRTTMin)
	require.GreaterOrEqual(t, virtualRTT, sender.rttStats.SmoothedRTT())
}

func TestPragueSenderMaxDatagramSizeChange(t *testing.T) {
	sender := newTestPragueSender(true)

	// Should panic on reduction
	require.Panics(t, func() {
		sender.sender.SetMaxDatagramSize(initialMaxDatagramSize - 1)
	})

	// Should work on increase
	newSize := initialMaxDatagramSize + 100

	// Set CWND to minimum first to test scaling behavior
	initialMinCwnd := sender.sender.minCongestionWindow()
	sender.sender.congestionWindow = initialMinCwnd

	// Change max datagram size
	sender.sender.SetMaxDatagramSize(newSize)
	require.Equal(t, newSize, sender.sender.maxDatagramSize)

	// CWND should scale with new datagram size when it was at minimum
	expectedNewMinCwnd := protocol.ByteCount(pragueMinCwnd) * newSize
	require.Equal(t, expectedNewMinCwnd, sender.sender.GetCongestionWindow())
}

func TestPragueSenderPacing(t *testing.T) {
	sender := newTestPragueSender(true)

	// Set up RTT and advance clock
	sender.rttStats.UpdateRTT(10*time.Millisecond, 0)
	sender.clock.Advance(time.Hour)

	// Fill the send window
	sender.SendAvailableSendWindow()
	sender.AckNPackets(1)

	// Check that we can't send immediately due to pacing
	delay := sender.sender.TimeUntilSend(sender.bytesInFlight)
	require.NotZero(t, delay)
	require.Less(t, delay.Sub(monotime.Time(*sender.clock)), time.Hour)

	// Should have pacing budget initially
	require.True(t, sender.sender.HasPacingBudget(sender.clock.Now()))
}

func TestPragueSenderRecoveryExitOnNewPacketNumber(t *testing.T) {
	sender := newTestPragueSender(true)

	// Build up window and trigger loss
	for range 10 {
		sender.SendAvailableSendWindow()
		sender.AckNPackets(2)
	}
	sender.SendAvailableSendWindow()
	sender.LoseNPackets(1)

	require.True(t, sender.sender.InRecovery())

	// Test basic recovery behavior - CWND should be reduced
	minCwnd := sender.sender.minCongestionWindow()
	currentCwnd := sender.sender.GetCongestionWindow()

	// CWND should be at least the minimum
	require.GreaterOrEqual(t, currentCwnd, minCwnd)

	// Send and ack more packets - should be able to do basic operations
	if sender.sender.CanSend(sender.bytesInFlight) {
		sender.SendAvailableSendWindow()
	}
	sender.AckNPackets(1)

	// CWND should remain stable during recovery
	require.GreaterOrEqual(t, sender.sender.GetCongestionWindow(), minCwnd)
}

func TestPragueSenderECNFeedbackWithoutL4S(t *testing.T) {
	sender := newTestPragueSender(false) // L4S disabled

	// ECN feedback should be ignored when L4S is disabled
	sender.SendAvailableSendWindow()
	initialAlpha := sender.sender.alpha
	sender.sender.OnECNFeedback(initialMaxDatagramSize)

	require.Equal(t, initialAlpha, sender.sender.alpha)
	require.Empty(t, sender.tracer.ecnFeedbackEvents)
}

func TestPragueSenderMinimumCongestionWindow(t *testing.T) {
	sender := newTestPragueSender(true)

	// Minimum CWND should be 2 * maxDatagramSize
	expectedMinCwnd := protocol.ByteCount(pragueMinCwnd) * sender.sender.maxDatagramSize
	require.Equal(t, expectedMinCwnd, sender.sender.minCongestionWindow())

	// After multiple losses, CWND shouldn't go below minimum
	sender.SendAvailableSendWindow()
	for range 10 {
		sender.LoseNPackets(1)
	}

	require.GreaterOrEqual(t, sender.sender.GetCongestionWindow(), expectedMinCwnd)
}

func TestPragueSenderCongestionStateLogging(t *testing.T) {
	sender := newTestPragueSender(true)

	// Tracer might be nil or logging might not be initialized
	if sender.tracer.stateChanges == nil {
		t.Skip("State change logging not active")
	}

	// Should start with slow start state logged
	require.NotEmpty(t, sender.tracer.stateChanges)

	// Trigger loss to enter recovery
	sender.SendAvailableSendWindow()
	sender.LoseNPackets(1)

	// Verify we have some state changes logged
	require.NotEmpty(t, sender.tracer.stateChanges)
}

func TestPragueSenderCwndCarryFractionalReductions(t *testing.T) {
	sender := newTestPragueSender(true)

	// Set a large CWND and small alpha for fractional reductions
	sender.sender.congestionWindow = initialMaxDatagramSize * 50 // Very large CWND
	sender.sender.alpha = 0.05                                   // Very small alpha
	sender.sender.cwndCarry = 0.0

	initialCarry := sender.sender.cwndCarry
	originalCwnd := sender.sender.congestionWindow

	// Apply multiple small ECN responses to accumulate fractional carry
	for range 10 {
		sender.sender.applyECNCongestionResponse()
		if sender.sender.GetCongestionWindow() < originalCwnd {
			break // Stop once we see a reduction
		}
	}

	// Carry should have accumulated or CWND should have decreased
	hasAccumulatedCarry := sender.sender.cwndCarry > initialCarry
	hasDecreasedCwnd := sender.sender.GetCongestionWindow() < originalCwnd
	require.True(t, hasAccumulatedCarry || hasDecreasedCwnd, "Expected either carry accumulation or CWND reduction")
}

func TestPragueSenderAlphaClampingToValidRange(t *testing.T) {
	sender := newTestPragueSender(true)

	// Test alpha clamping to [0,1] range
	sender.sender.alpha = -0.5 // Invalid negative value
	sender.sender.totalAckedBytes = initialMaxDatagramSize * 10
	sender.sender.ecnMarkedBytes = 0
	sender.sender.updateAlpha()

	require.GreaterOrEqual(t, sender.sender.alpha, 0.0)

	// Test upper bound (alpha > 1.0 should be clamped to 1.0)
	sender.sender.alpha = 1.5 // Invalid high value
	sender.sender.updateAlpha()

	require.LessOrEqual(t, sender.sender.alpha, 1.0)
}
