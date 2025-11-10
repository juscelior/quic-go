package congestion

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

// BenchmarkPragueAlgorithmCreation benchmarks Prague algorithm creation
func BenchmarkPragueAlgorithmCreation(b *testing.B) {
	b.ReportAllocs()

	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	for b.Loop() {
		sender := NewPragueSender(
			clock,
			rttStats,
			connStats,
			protocol.InitialPacketSize,
			true, // L4S enabled
		)
		_ = sender
	}
}

// BenchmarkPraguePacketSent benchmarks OnPacketSent method
func BenchmarkPraguePacketSent(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sentTime := monotime.Now()
	bytesInFlight := protocol.ByteCount(1000)
	packetNumber := protocol.PacketNumber(1)
	packetSize := protocol.ByteCount(1200)

	for b.Loop() {
		sender.OnPacketSent(sentTime, bytesInFlight, packetNumber, packetSize, true)
		packetNumber++
	}
}

// BenchmarkPraguePacketAcked benchmarks OnPacketAcked method
func BenchmarkPraguePacketAcked(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	ackedBytes := protocol.ByteCount(1200)
	priorInFlight := protocol.ByteCount(5000)
	eventTime := monotime.Now()

	for b.Loop() {
		sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), ackedBytes, priorInFlight, eventTime)
	}
}

// BenchmarkPragueAlphaCalculation benchmarks alpha parameter calculation
func BenchmarkPragueAlphaCalculation(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.l4sEnabled = true

	// Set up scenario for alpha calculation
	sender.totalAckedBytes = 10000
	sender.ecnMarkedBytes = 500 // 5% marking rate

	for b.Loop() {
		sender.updateAlpha()
	}
}

// BenchmarkPragueECNCongestionResponse benchmarks ECN congestion response
func BenchmarkPragueECNCongestionResponse(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.alpha = 0.1                                  // 10% alpha
	sender.congestionWindow = protocol.ByteCount(20000) // 20KB window

	for b.Loop() {
		sender.applyECNCongestionResponse()
		// Reset window for next iteration
		sender.congestionWindow = protocol.ByteCount(20000)
	}
}

// BenchmarkPragueAdditiveIncrease benchmarks additive increase calculation
func BenchmarkPragueAdditiveIncrease(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.alpha = 0.05 // 5% alpha
	sender.congestionWindow = protocol.ByteCount(10000)
	ackedBytes := protocol.ByteCount(1200)

	for b.Loop() {
		sender.pragueAdditiveIncrease(ackedBytes)
	}
}

// BenchmarkPragueECNFeedback benchmarks full ECN feedback processing
func BenchmarkPragueECNFeedback(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.l4sEnabled = true
	ecnMarkedBytes := protocol.ByteCount(600) // Some ECN marked bytes

	for b.Loop() {
		sender.OnECNFeedback(ecnMarkedBytes)
	}
}

// BenchmarkPragueCongestionEvent benchmarks loss event processing
func BenchmarkPragueCongestionEvent(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	packetNumber := protocol.PacketNumber(100)
	lostBytes := protocol.ByteCount(1200)
	priorInFlight := protocol.ByteCount(8000)

	for b.Loop() {
		sender.OnCongestionEvent(packetNumber, lostBytes, priorInFlight)
		packetNumber++
		// Reset state for next iteration
		sender.inRecovery = false
		sender.largestSentAtLastCutback = protocol.InvalidPacketNumber
	}
}

// BenchmarkPragueBandwidthEstimate benchmarks bandwidth calculation
func BenchmarkPragueBandwidthEstimate(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	// Set up RTT for bandwidth calculation
	sender.rttStats.UpdateRTT(50*time.Millisecond, 0)

	for b.Loop() {
		bandwidth := sender.BandwidthEstimate()
		_ = bandwidth
	}
}

// BenchmarkPragueCanSend benchmarks CanSend check
func BenchmarkPragueCanSend(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	bytesInFlight := protocol.ByteCount(5000)

	for b.Loop() {
		canSend := sender.CanSend(bytesInFlight)
		_ = canSend
	}
}

// BenchmarkPragueVirtualRTT benchmarks virtual RTT calculation
func BenchmarkPragueVirtualRTT(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.rttStats.UpdateRTT(30*time.Millisecond, 0)

	for b.Loop() {
		vRTT := sender.getVirtualRTT()
		_ = vRTT
	}
}

// BenchmarkPragueFullCongestionLoop benchmarks a complete congestion control loop
func BenchmarkPragueFullCongestionLoop(b *testing.B) {
	b.ReportAllocs()

	sender := createBenchmarkPragueSender()
	sender.l4sEnabled = true

	// Simulate a complete congestion control cycle
	sentTime := monotime.Now()
	bytesInFlight := protocol.ByteCount(1000)
	packetSize := protocol.ByteCount(1200)

	for b.Loop() {
		packetNumber := protocol.PacketNumber(b.Elapsed() + 1)

		// 1. Send packet
		sender.OnPacketSent(sentTime, bytesInFlight, packetNumber, packetSize, true)

		// 2. ACK packet (90% of the time)
		if b.Elapsed()%10 != 0 {
			sender.OnPacketAcked(packetNumber, packetSize, bytesInFlight, sentTime.Add(30*time.Millisecond))
		}

		// 3. ECN feedback (10% of packets marked)
		if b.Elapsed()%10 == 0 {
			sender.OnECNFeedback(packetSize / 10)
		}

		// 4. Loss event (1% of packets)
		if b.Elapsed()%100 == 0 {
			sender.OnCongestionEvent(packetNumber, packetSize, bytesInFlight)
		}

		bytesInFlight += packetSize / 2 // Simulate partial flight size changes
	}
}

// BenchmarkPragueVsCubicCreation compares Prague vs CUBIC/RFC9002 creation
func BenchmarkPragueVsCubicCreation(b *testing.B) {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	b.Run("Prague", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sender := NewPragueSender(clock, rttStats, connStats, protocol.InitialPacketSize, true)
			_ = sender
		}
	})

	b.Run("RFC9002", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sender := NewCubicSender(clock, rttStats, connStats, protocol.InitialPacketSize, false, nil)
			_ = sender
		}
	})
}

// BenchmarkPragueVsCubicPacketProcessing compares packet processing performance
func BenchmarkPragueVsCubicPacketProcessing(b *testing.B) {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	pragueSender := NewPragueSender(clock, rttStats, connStats, protocol.InitialPacketSize, true)
	cubicSender := NewCubicSender(clock, rttStats, connStats, protocol.InitialPacketSize, false, nil)

	sentTime := monotime.Now()
	bytesInFlight := protocol.ByteCount(5000)
	packetSize := protocol.ByteCount(1200)

	b.Run("Prague-OnPacketAcked", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pragueSender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), packetSize, bytesInFlight, sentTime)
		}
	})

	b.Run("RFC9002-OnPacketAcked", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			cubicSender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), packetSize, bytesInFlight, sentTime)
		}
	})
}

// BenchmarkPragueWithTracing benchmarks Prague with tracing enabled
func BenchmarkPragueWithTracing(b *testing.B) {
	var alphaUpdates int
	var ecnEvents int

	b.Run("With-Tracing", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true

		for b.Loop() {
			sender.OnECNFeedback(protocol.ByteCount(100))
		}
	})

	b.Run("Without-Tracing", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true

		for b.Loop() {
			sender.OnECNFeedback(protocol.ByteCount(100))
		}
	})

	b.Logf("Alpha updates: %d, ECN events: %d", alphaUpdates, ecnEvents)
}

// Helper function to create a Prague sender for benchmarking
func createBenchmarkPragueSender() *pragueSender {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	rttStats.UpdateRTT(50*time.Millisecond, 0)

	sender := NewPragueSender(
		clock,
		rttStats,
		connStats,
		protocol.InitialPacketSize,
		true, // L4S enabled
	)

	sender.congestionWindow = protocol.ByteCount(10000)
	sender.largestSentPacketNumber = 50
	sender.largestAckedPacketNumber = 45

	return sender
}
