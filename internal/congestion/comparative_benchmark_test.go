package congestion

import (
	"fmt"
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

// BenchmarkAlgorithmComparison compares Prague vs RFC9002 across multiple scenarios
func BenchmarkAlgorithmComparison(b *testing.B) {
	scenarios := []struct {
		name        string
		cwnd        protocol.ByteCount
		rtt         time.Duration
		packetSize  protocol.ByteCount
		description string
	}{
		{"SmallWindow", 4000, 10 * time.Millisecond, 1200, "Small congestion window, low RTT"},
		{"MediumWindow", 20000, 50 * time.Millisecond, 1200, "Medium congestion window, medium RTT"},
		{"LargeWindow", 100000, 100 * time.Millisecond, 1200, "Large congestion window, high RTT"},
		{"HighBandwidth", 200000, 20 * time.Millisecond, 1400, "High bandwidth scenario"},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.Run("Prague", func(b *testing.B) {
				benchmarkAlgorithmScenario(b, "prague", scenario.cwnd, scenario.rtt, scenario.packetSize)
			})
			b.Run("RFC9002", func(b *testing.B) {
				benchmarkAlgorithmScenario(b, "rfc9002", scenario.cwnd, scenario.rtt, scenario.packetSize)
			})
		})
	}
}

// BenchmarkCongestionResponse compares congestion response performance
func BenchmarkCongestionResponse(b *testing.B) {
	b.Run("Prague-ECN-Response", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.alpha = 0.1
		sender.congestionWindow = 50000

		for b.Loop() {
			sender.applyECNCongestionResponse()
			sender.congestionWindow = 50000 // Reset for next iteration
		}
	})

	b.Run("Prague-Loss-Response", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		packetNumber := protocol.PacketNumber(100)
		lostBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(10000)

		for b.Loop() {
			sender.OnCongestionEvent(packetNumber, lostBytes, priorInFlight)
			packetNumber++
			// Reset state
			sender.inRecovery = false
			sender.largestSentAtLastCutback = protocol.InvalidPacketNumber
			sender.congestionWindow = 50000
		}
	})

	b.Run("RFC9002-Loss-Response", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkCubicSender()
		packetNumber := protocol.PacketNumber(100)
		lostBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(10000)

		for b.Loop() {
			sender.OnCongestionEvent(packetNumber, lostBytes, priorInFlight)
			packetNumber++
			// Reset state
			sender.largestSentAtLastCutback = protocol.InvalidPacketNumber
			sender.congestionWindow = 50000
		}
	})
}

// BenchmarkBandwidthCalculation compares bandwidth estimation performance
func BenchmarkBandwidthCalculation(b *testing.B) {
	rtts := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}

	for _, rtt := range rtts {
		b.Run(fmt.Sprintf("RTT-%dms", rtt.Milliseconds()), func(b *testing.B) {
			b.Run("Prague", func(b *testing.B) {
				b.ReportAllocs()
				sender := createBenchmarkPragueSender()
				sender.rttStats.UpdateRTT(rtt, 0)

				for b.Loop() {
					bw := sender.BandwidthEstimate()
					_ = bw
				}
			})

			b.Run("RFC9002", func(b *testing.B) {
				b.ReportAllocs()
				sender := createBenchmarkCubicSender()
				sender.rttStats.UpdateRTT(rtt, 0)

				for b.Loop() {
					bw := sender.BandwidthEstimate()
					_ = bw
				}
			})
		})
	}
}

// BenchmarkSlowStartPerformance compares slow start performance
func BenchmarkSlowStartPerformance(b *testing.B) {
	b.Run("Prague-SlowStart", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.inSlowStart = true
		sender.congestionWindow = 2000 // Start small

		ackedBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(1000)
		eventTime := monotime.Now()

		for b.Loop() {
			if !sender.inSlowStart {
				// Reset for next iteration
				sender.inSlowStart = true
				sender.congestionWindow = 2000
			}
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), ackedBytes, priorInFlight, eventTime)
		}
	})

	b.Run("RFC9002-SlowStart", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkCubicSender()
		sender.congestionWindow = 2000 // Start small
		sender.slowStartThreshold = protocol.MaxByteCount

		ackedBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(1000)
		eventTime := monotime.Now()

		for b.Loop() {
			if !sender.InSlowStart() {
				// Reset for next iteration
				sender.congestionWindow = 2000
				sender.slowStartThreshold = protocol.MaxByteCount
			}
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), ackedBytes, priorInFlight, eventTime)
		}
	})
}

// BenchmarkMemoryUsage compares memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Prague-MemoryFootprint", func(b *testing.B) {
		b.ReportAllocs()

		var senders []*pragueSender
		for b.Loop() {
			sender := createBenchmarkPragueSender()
			senders = append(senders, sender)
			if len(senders) > 1000 {
				senders = senders[:0] // Clear slice to avoid OOM
			}
		}
		_ = senders
	})

	b.Run("RFC9002-MemoryFootprint", func(b *testing.B) {
		b.ReportAllocs()

		var senders []*cubicSender
		for b.Loop() {
			sender := createBenchmarkCubicSender()
			senders = append(senders, sender)
			if len(senders) > 1000 {
				senders = senders[:0] // Clear slice to avoid OOM
			}
		}
		_ = senders
	})
}

// BenchmarkHighThroughputScenario simulates high throughput scenarios
func BenchmarkHighThroughputScenario(b *testing.B) {
	const packetsPerLoop = 100

	b.Run("Prague-HighThroughput", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.congestionWindow = 100000 // Large window
		sender.l4sEnabled = true

		sentTime := monotime.Now()
		packetSize := protocol.ByteCount(1400)

		for b.Loop() {
			for i := 0; i < packetsPerLoop; i++ {
				packetNumber := protocol.PacketNumber(int64(b.Elapsed())*packetsPerLoop + int64(i))

				// Send packet
				sender.OnPacketSent(sentTime, 50000, packetNumber, packetSize, true)

				// ACK packet
				sender.OnPacketAcked(packetNumber, packetSize, 50000, sentTime.Add(10*time.Millisecond))

				// Occasional ECN feedback
				if i%10 == 0 {
					sender.OnECNFeedback(packetSize / 20) // 5% marking
				}
			}
		}
	})

	b.Run("RFC9002-HighThroughput", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkCubicSender()
		sender.congestionWindow = 100000 // Large window

		sentTime := monotime.Now()
		packetSize := protocol.ByteCount(1400)

		for b.Loop() {
			for i := 0; i < packetsPerLoop; i++ {
				packetNumber := protocol.PacketNumber(int64(b.Elapsed())*packetsPerLoop + int64(i))

				// Send packet
				sender.OnPacketSent(sentTime, 50000, packetNumber, packetSize, true)

				// ACK packet
				sender.OnPacketAcked(packetNumber, packetSize, 50000, sentTime.Add(10*time.Millisecond))
			}
		}
	})
}

// BenchmarkAlgorithmStates compares different algorithm states
func BenchmarkAlgorithmStates(b *testing.B) {
	states := []struct {
		name        string
		setupPrague func(*pragueSender)
		setupCubic  func(*cubicSender)
	}{
		{
			name: "SlowStart",
			setupPrague: func(s *pragueSender) {
				s.inSlowStart = true
				s.congestionWindow = 5000
			},
			setupCubic: func(s *cubicSender) {
				s.congestionWindow = 5000
				s.slowStartThreshold = protocol.MaxByteCount
			},
		},
		{
			name: "CongestionAvoidance",
			setupPrague: func(s *pragueSender) {
				s.inSlowStart = false
				s.congestionWindow = 50000
				s.alpha = 0.05
			},
			setupCubic: func(s *cubicSender) {
				s.congestionWindow = 50000
				s.slowStartThreshold = 40000
			},
		},
		{
			name: "Recovery",
			setupPrague: func(s *pragueSender) {
				s.inRecovery = true
				s.congestionWindow = 20000
				s.largestSentAtLastCutback = 100
				s.largestAckedPacketNumber = 50
			},
			setupCubic: func(s *cubicSender) {
				s.congestionWindow = 20000
				s.largestSentAtLastCutback = 100
				s.largestAckedPacketNumber = 50
			},
		},
	}

	for _, state := range states {
		b.Run(state.name, func(b *testing.B) {
			b.Run("Prague", func(b *testing.B) {
				b.ReportAllocs()
				sender := createBenchmarkPragueSender()
				state.setupPrague(sender)

				ackedBytes := protocol.ByteCount(1200)
				priorInFlight := protocol.ByteCount(25000)
				eventTime := monotime.Now()

				for b.Loop() {
					sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()+200), ackedBytes, priorInFlight, eventTime)
				}
			})

			b.Run("RFC9002", func(b *testing.B) {
				b.ReportAllocs()
				sender := createBenchmarkCubicSender()
				state.setupCubic(sender)

				ackedBytes := protocol.ByteCount(1200)
				priorInFlight := protocol.ByteCount(25000)
				eventTime := monotime.Now()

				for b.Loop() {
					sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()+200), ackedBytes, priorInFlight, eventTime)
				}
			})
		})
	}
}

// BenchmarkPrecisionComparison compares floating point vs integer operations
func BenchmarkPrecisionComparison(b *testing.B) {
	b.Run("Prague-FloatingPoint", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.alpha = 0.123456789
		sender.congestionWindow = 23456

		for b.Loop() {
			// Test floating point operations in Prague
			sender.applyECNCongestionResponse()
			sender.congestionWindow = 23456 // Reset
		}
	})

	b.Run("RFC9002-IntegerMath", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkCubicSender()
		sender.congestionWindow = 23456

		for b.Loop() {
			// Test integer operations in RFC9002
			packetNumber := protocol.PacketNumber(b.Elapsed() + 100)
			sender.OnCongestionEvent(packetNumber, 1200, 25000)
			sender.congestionWindow = 23456 // Reset
			sender.largestSentAtLastCutback = protocol.InvalidPacketNumber
		}
	})
}

// Helper functions for creating benchmark senders
func benchmarkAlgorithmScenario(b *testing.B, algorithm string, cwnd protocol.ByteCount, rtt time.Duration, packetSize protocol.ByteCount) {
	b.ReportAllocs()

	if algorithm == "prague" {
		sender := createBenchmarkPragueSender()
		sender.congestionWindow = cwnd
		sender.rttStats.UpdateRTT(rtt, 0)

		priorInFlight := cwnd / 2
		eventTime := monotime.Now()

		for b.Loop() {
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), packetSize, priorInFlight, eventTime)
		}
	} else {
		sender := createBenchmarkCubicSender()
		sender.congestionWindow = cwnd
		sender.rttStats.UpdateRTT(rtt, 0)

		priorInFlight := cwnd / 2
		eventTime := monotime.Now()

		for b.Loop() {
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), packetSize, priorInFlight, eventTime)
		}
	}
}

func createBenchmarkCubicSender() *cubicSender {
	clock := DefaultClock{}
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	rttStats.UpdateRTT(50*time.Millisecond, 0)

	sender := NewCubicSender(
		clock,
		rttStats,
		connStats,
		protocol.InitialPacketSize,
		false, // not reno
		nil,   // no tracer
	)

	sender.congestionWindow = protocol.ByteCount(10000)
	sender.largestSentPacketNumber = 50
	sender.largestAckedPacketNumber = 45

	return sender
}

// BenchmarkRealWorldMix simulates real-world mixed traffic patterns
func BenchmarkRealWorldMix(b *testing.B) {
	b.Run("Prague-RealWorld", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true

		benchmarkRealWorldPattern(b, "prague", sender, nil)
	})

	b.Run("RFC9002-RealWorld", func(b *testing.B) {
		b.ReportAllocs()
		cubicSender := createBenchmarkCubicSender()

		benchmarkRealWorldPattern(b, "rfc9002", nil, cubicSender)
	})
}

func benchmarkRealWorldPattern(b *testing.B, algorithm string, pragueSender *pragueSender, cubicSender *cubicSender) {
	sentTime := monotime.Now()
	packetSize := protocol.ByteCount(1200)

	for b.Loop() {
		iteration := b.Elapsed()
		packetNumber := protocol.PacketNumber(iteration)
		bytesInFlight := protocol.ByteCount(5000 + (iteration%10)*1000)

		// 1. Send packet (always)
		if algorithm == "prague" {
			pragueSender.OnPacketSent(sentTime, bytesInFlight, packetNumber, packetSize, true)
		} else {
			cubicSender.OnPacketSent(sentTime, bytesInFlight, packetNumber, packetSize, true)
		}

		// 2. ACK packet (95% success rate)
		if iteration%20 != 0 {
			ackTime := sentTime.Add(time.Duration(20+iteration%30) * time.Millisecond)
			if algorithm == "prague" {
				pragueSender.OnPacketAcked(packetNumber, packetSize, bytesInFlight, ackTime)
			} else {
				cubicSender.OnPacketAcked(packetNumber, packetSize, bytesInFlight, ackTime)
			}
		}

		// 3. ECN feedback (Prague only, 8% of packets)
		if algorithm == "prague" && iteration%12 == 0 {
			ecnMarked := packetSize / 25 // 4% marking rate
			pragueSender.OnECNFeedback(ecnMarked)
		}

		// 4. Loss event (0.5% of packets)
		if iteration%200 == 0 {
			if algorithm == "prague" {
				pragueSender.OnCongestionEvent(packetNumber, packetSize, bytesInFlight)
			} else {
				cubicSender.OnCongestionEvent(packetNumber, packetSize, bytesInFlight)
			}
		}

		// 5. RTT variation
		if iteration%50 == 0 {
			newRTT := time.Duration(40+iteration%40) * time.Millisecond
			if algorithm == "prague" {
				pragueSender.rttStats.UpdateRTT(newRTT, 0)
			} else {
				cubicSender.rttStats.UpdateRTT(newRTT, 0)
			}
		}
	}
}
