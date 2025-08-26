package congestion

import (
	"fmt"
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/logging"
)

// BenchmarkECNMarkingOverhead measures the overhead of ECN marking in L4S
func BenchmarkECNMarkingOverhead(b *testing.B) {
	b.Run("L4S-ECN-Enabled", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true
		
		ecnMarkedBytes := protocol.ByteCount(120) // 10% of 1200 byte packet
		
		for b.Loop() {
			sender.OnECNFeedback(ecnMarkedBytes)
		}
	})
	
	b.Run("L4S-ECN-Disabled", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = false
		
		ecnMarkedBytes := protocol.ByteCount(120)
		
		for b.Loop() {
			sender.OnECNFeedback(ecnMarkedBytes)
		}
	})
	
	b.Run("No-ECN-Processing", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		
		ackedBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(5000)
		eventTime := time.Now()
		
		for b.Loop() {
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), ackedBytes, priorInFlight, eventTime)
		}
	})
}

// BenchmarkAlphaCalculationOverhead measures alpha calculation overhead
func BenchmarkAlphaCalculationOverhead(b *testing.B) {
	scenarios := []struct {
		name         string
		markingRate  float64
		description  string
	}{
		{"NoMarking", 0.0, "No ECN marking (0%)"},
		{"LightMarking", 0.01, "Light ECN marking (1%)"},
		{"ModerateMarking", 0.05, "Moderate ECN marking (5%)"},
		{"HeavyMarking", 0.15, "Heavy ECN marking (15%)"},
		{"ExtremeMarking", 0.30, "Extreme ECN marking (30%)"},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.l4sEnabled = true
			
			totalBytes := protocol.ByteCount(10000)
			markedBytes := protocol.ByteCount(float64(totalBytes) * scenario.markingRate)
			
			for b.Loop() {
				sender.totalAckedBytes = totalBytes
				sender.ecnMarkedBytes = markedBytes
				sender.updateAlpha()
			}
		})
	}
}

// BenchmarkECNFeedbackFrequency measures overhead at different feedback frequencies
func BenchmarkECNFeedbackFrequency(b *testing.B) {
	frequencies := []struct {
		name     string
		interval int
		desc     string
	}{
		{"EveryPacket", 1, "ECN feedback every packet"},
		{"Every5Packets", 5, "ECN feedback every 5 packets"},
		{"Every10Packets", 10, "ECN feedback every 10 packets"},
		{"Every20Packets", 20, "ECN feedback every 20 packets"},
	}

	for _, freq := range frequencies {
		b.Run(freq.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.l4sEnabled = true
			
			packetSize := protocol.ByteCount(1200)
			ecnMarkedBytes := packetSize / 10 // 10% marking
			
			for b.Loop() {
				iteration := b.Elapsed()
				
				// Regular packet processing
				sender.OnPacketAcked(protocol.PacketNumber(iteration), packetSize, 5000, time.Now())
				
				// ECN feedback at specified frequency
				if int(iteration)%freq.interval == 0 {
					sender.OnECNFeedback(ecnMarkedBytes)
				}
			}
		})
	}
}

// BenchmarkECNTracingOverhead measures tracing overhead
func BenchmarkECNTracingOverhead(b *testing.B) {
	createTracingBenchmark := func(withTracing bool) func(*testing.B) {
		return func(b *testing.B) {
			b.ReportAllocs()
			
			var tracer *logging.ConnectionTracer
			if withTracing {
				var alphaUpdates, ecnEvents int
				tracer = &logging.ConnectionTracer{
					UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
						alphaUpdates++
					},
					PragueECNFeedback: func(ecnMarkedBytes, totalBytes protocol.ByteCount) {
						ecnEvents++
					},
				}
			}
			
			sender := createBenchmarkPragueSenderWithTracer(tracer)
			sender.l4sEnabled = true
			
			ecnMarkedBytes := protocol.ByteCount(60) // 5% of 1200 bytes
			
			for b.Loop() {
				sender.OnECNFeedback(ecnMarkedBytes)
			}
		}
	}
	
	b.Run("WithTracing", createTracingBenchmark(true))
	b.Run("WithoutTracing", createTracingBenchmark(false))
}

// BenchmarkECNStateManagement measures ECN state management overhead
func BenchmarkECNStateManagement(b *testing.B) {
	b.Run("L4S-StateUpdates", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true
		
		// Simulate varying ECN conditions
		markingRates := []float64{0.0, 0.02, 0.05, 0.1, 0.15, 0.08, 0.03, 0.01}
		
		for b.Loop() {
			iteration := b.Elapsed()
			markingRate := markingRates[int(iteration)%len(markingRates)]
			
			totalBytes := protocol.ByteCount(5000)
			markedBytes := protocol.ByteCount(float64(totalBytes) * markingRate)
			
			sender.totalAckedBytes = totalBytes
			sender.ecnMarkedBytes = markedBytes
			sender.updateAlpha()
			sender.applyECNCongestionResponse()
			
			// Reset for next iteration
			sender.congestionWindow = 20000
		}
	})
	
	b.Run("NoL4S-NoStateUpdates", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = false
		
		for b.Loop() {
			// Just regular packet ack without ECN processing
			sender.OnPacketAcked(protocol.PacketNumber(b.Elapsed()), 1200, 5000, time.Now())
		}
	})
}

// BenchmarkECNMemoryAccess measures memory access patterns for ECN
func BenchmarkECNMemoryAccess(b *testing.B) {
	b.Run("L4S-MemoryPattern", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true
		
		for b.Loop() {
			// Access ECN-specific fields
			_ = sender.alpha
			_ = sender.ecnMarkedBytes
			_ = sender.totalAckedBytes
			_ = sender.cwndCarry
			_ = sender.l4sEnabled
			
			// Perform ECN calculation
			if sender.totalAckedBytes > 0 {
				markingFraction := float64(sender.ecnMarkedBytes) / float64(sender.totalAckedBytes)
				sender.alpha = (1.0-sender.alphaGain)*sender.alpha + sender.alphaGain*markingFraction
			}
		}
	})
	
	b.Run("RFC9002-MemoryPattern", func(b *testing.B) {
		b.ReportAllocs()
		cubicSender := createBenchmarkCubicSender()
		
		for b.Loop() {
			// Access RFC9002-specific fields
			_ = cubicSender.congestionWindow
			_ = cubicSender.slowStartThreshold
			_ = cubicSender.largestAckedPacketNumber
			_ = cubicSender.largestSentPacketNumber
			_ = cubicSender.numAckedPackets
			
			// Perform basic calculation
			if cubicSender.congestionWindow > 0 {
				_ = cubicSender.congestionWindow / cubicSender.maxDatagramSize
			}
		}
	})
}

// BenchmarkECNPacketSizeImpact measures impact of packet size on ECN processing
func BenchmarkECNPacketSizeImpact(b *testing.B) {
	packetSizes := []protocol.ByteCount{
		576,   // Minimum MTU
		1200,  // Typical packet size
		1400,  // Near MTU limit
		9000,  // Jumbo frame
	}

	for _, packetSize := range packetSizes {
		b.Run(fmt.Sprintf("PacketSize-%d", packetSize), func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.l4sEnabled = true
			
			// 5% marking rate
			ecnMarkedBytes := packetSize / 20
			
			for b.Loop() {
				sender.OnECNFeedback(ecnMarkedBytes)
			}
		})
	}
}

// BenchmarkECNAlgorithmScaling measures how ECN processing scales
func BenchmarkECNAlgorithmScaling(b *testing.B) {
	windowSizes := []protocol.ByteCount{
		2000,   // Small window
		10000,  // Medium window  
		50000,  // Large window
		200000, // Very large window
	}

	for _, windowSize := range windowSizes {
		b.Run(fmt.Sprintf("Window-%d", windowSize), func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.l4sEnabled = true
			sender.congestionWindow = windowSize
			sender.alpha = 0.05 // Fixed alpha
			
			for b.Loop() {
				sender.applyECNCongestionResponse()
				sender.congestionWindow = windowSize // Reset
			}
		})
	}
}

// BenchmarkECNRealWorldScenario simulates real-world ECN marking patterns
func BenchmarkECNRealWorldScenario(b *testing.B) {
	b.Run("L4S-RealWorld", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true
		
		// Simulate varying network conditions
		for b.Loop() {
			iteration := b.Elapsed()
			
			// Network congestion varies over time
			congestionLevel := float64((iteration % 100)) / 100.0
			markingRate := congestionLevel * 0.2 // 0% to 20% marking
			
			packetSize := protocol.ByteCount(1200)
			markedBytes := protocol.ByteCount(float64(packetSize) * markingRate)
			
			// Regular packet processing
			sender.OnPacketAcked(protocol.PacketNumber(iteration), packetSize, 5000, time.Now())
			
			// ECN feedback every few packets
			if iteration%5 == 0 {
				sender.OnECNFeedback(markedBytes)
			}
			
			// Occasional loss events
			if iteration%200 == 0 {
				sender.OnCongestionEvent(protocol.PacketNumber(iteration), packetSize, 5000)
			}
		}
	})
	
	b.Run("RFC9002-Baseline", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkCubicSender()
		
		for b.Loop() {
			iteration := b.Elapsed()
			packetSize := protocol.ByteCount(1200)
			
			// Regular packet processing (no ECN)
			sender.OnPacketAcked(protocol.PacketNumber(iteration), packetSize, 5000, time.Now())
			
			// Occasional loss events
			if iteration%200 == 0 {
				sender.OnCongestionEvent(protocol.PacketNumber(iteration), packetSize, 5000)
			}
		}
	})
}

// Helper function to format ECN overhead results
func formatECNOverhead(pragueNs, rfc9002Ns float64) string {
	if rfc9002Ns == 0 {
		return "N/A"
	}
	overhead := ((pragueNs - rfc9002Ns) / rfc9002Ns) * 100
	return fmt.Sprintf("%.1f%%", overhead)
}