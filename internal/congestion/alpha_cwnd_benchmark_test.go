package congestion

import (
	"fmt"
	"testing"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

// BenchmarkAlphaCalculationPrecision benchmarks alpha calculation with different precision levels
func BenchmarkAlphaCalculationPrecision(b *testing.B) {
	precisionTests := []struct {
		name        string
		alphaGain   float64
		description string
	}{
		{"DefaultGain", pragueAlphaGain, "Default alpha gain (1/16)"},
		{"HighGain", 0.125, "High alpha gain (1/8)"},
		{"LowGain", 0.03125, "Low alpha gain (1/32)"},
		{"VeryLowGain", 0.015625, "Very low alpha gain (1/64)"},
	}

	for _, test := range precisionTests {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.alphaGain = test.alphaGain
			sender.l4sEnabled = true

			// Simulate realistic marking scenarios
			totalBytes := protocol.ByteCount(12000) // 10 packets
			markedBytes := protocol.ByteCount(600)  // 5% marking

			for b.Loop() {
				sender.totalAckedBytes = totalBytes
				sender.ecnMarkedBytes = markedBytes
				sender.updateAlpha()
			}
		})
	}
}

// BenchmarkCWNDUpdateMechanisms benchmarks different CWND update mechanisms
func BenchmarkCWNDUpdateMechanisms(b *testing.B) {
	b.Run("Prague-SlowStart", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.inSlowStart = true
		sender.congestionWindow = 4000

		ackedBytes := protocol.ByteCount(1200)

		for b.Loop() {
			if sender.congestionWindow > 50000 {
				sender.congestionWindow = 4000 // Reset
				sender.inSlowStart = true
			}

			// Slow start: cwnd += ackedBytes
			sender.congestionWindow += ackedBytes
			sender.MaybeExitSlowStart()
		}
	})

	b.Run("Prague-AdditiveIncrease", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.inSlowStart = false
		sender.alpha = 0.05 // 5% alpha
		sender.congestionWindow = 20000

		ackedBytes := protocol.ByteCount(1200)

		for b.Loop() {
			sender.pragueAdditiveIncrease(ackedBytes)
		}
	})

	b.Run("Prague-ECN-Response", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.alpha = 0.08 // 8% alpha
		sender.congestionWindow = 30000

		for b.Loop() {
			sender.applyECNCongestionResponse()
			sender.congestionWindow = 30000 // Reset
		}
	})

	b.Run("Prague-Loss-Response", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.congestionWindow = 40000

		packetNumber := protocol.PacketNumber(100)
		lostBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(20000)

		for b.Loop() {
			sender.OnCongestionEvent(packetNumber, lostBytes, priorInFlight)
			packetNumber++
			// Reset state
			sender.congestionWindow = 40000
			sender.inRecovery = false
			sender.largestSentAtLastCutback = protocol.InvalidPacketNumber
		}
	})
}

// BenchmarkCWNDCarryMechanism benchmarks the fractional CWND carry logic
func BenchmarkCWNDCarryMechanism(b *testing.B) {
	carryScenarios := []struct {
		name         string
		alpha        float64
		initialCarry float64
		description  string
	}{
		{"SmallAlpha", 0.02, 0.0, "Small alpha with no carry"},
		{"MediumAlpha", 0.08, 0.0, "Medium alpha with no carry"},
		{"LargeAlpha", 0.15, 0.0, "Large alpha with no carry"},
		{"WithCarry", 0.05, 0.7, "Medium alpha with existing carry"},
		{"HighCarry", 0.03, 0.9, "Small alpha with high carry"},
	}

	for _, scenario := range carryScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.alpha = scenario.alpha
			sender.cwndCarry = scenario.initialCarry
			sender.congestionWindow = 25000

			for b.Loop() {
				initialCwnd := sender.congestionWindow
				sender.applyECNCongestionResponse()

				// Reset for next iteration
				sender.congestionWindow = initialCwnd
				if sender.cwndCarry < 0.1 {
					sender.cwndCarry = scenario.initialCarry
				}
			}
		})
	}
}

// BenchmarkAlphaConvergence benchmarks alpha convergence under different conditions
func BenchmarkAlphaConvergence(b *testing.B) {
	convergenceTests := []struct {
		name          string
		initialAlpha  float64
		targetMarking float64
		description   string
	}{
		{"ColdStart", 0.0, 0.05, "Cold start to 5% marking"},
		{"LowToHigh", 0.02, 0.15, "Low to high marking transition"},
		{"HighToLow", 0.20, 0.03, "High to low marking transition"},
		{"Oscillating", 0.10, 0.10, "Stable marking conditions"},
	}

	for _, test := range convergenceTests {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.alpha = test.initialAlpha
			sender.l4sEnabled = true

			totalBytes := protocol.ByteCount(10000)
			targetMarkedBytes := protocol.ByteCount(float64(totalBytes) * test.targetMarking)

			for b.Loop() {
				sender.totalAckedBytes = totalBytes
				sender.ecnMarkedBytes = targetMarkedBytes
				sender.updateAlpha()
			}
		})
	}
}

// BenchmarkCWNDScaling benchmarks CWND operations at different scales
func BenchmarkCWNDScaling(b *testing.B) {
	scales := []struct {
		name        string
		windowSize  protocol.ByteCount
		description string
	}{
		{"Tiny", 2400, "Tiny window (2 packets)"},
		{"Small", 12000, "Small window (10 packets)"},
		{"Medium", 60000, "Medium window (50 packets)"},
		{"Large", 300000, "Large window (250 packets)"},
		{"Huge", 1200000, "Huge window (1000 packets)"},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.congestionWindow = scale.windowSize
			sender.alpha = 0.06 // 6% alpha

			for b.Loop() {
				initialCwnd := sender.congestionWindow
				sender.applyECNCongestionResponse()
				sender.congestionWindow = initialCwnd // Reset
			}
		})
	}
}

// BenchmarkAlphaEWMAPerformance benchmarks EWMA calculation performance
func BenchmarkAlphaEWMAPerformance(b *testing.B) {
	ewmaTests := []struct {
		name         string
		currentAlpha float64
		markingRate  float64
		description  string
	}{
		{"ZeroAlpha", 0.0, 0.05, "Starting from zero alpha"},
		{"LowAlpha", 0.02, 0.08, "Low alpha with higher marking"},
		{"HighAlpha", 0.25, 0.15, "High alpha with medium marking"},
		{"SteadyState", 0.05, 0.05, "Steady state conditions"},
	}

	for _, test := range ewmaTests {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.alpha = test.currentAlpha

			// Pre-calculate values for benchmark
			totalBytes := protocol.ByteCount(8000)
			markingFraction := test.markingRate
			gain := sender.alphaGain
			_ = totalBytes

			for b.Loop() {
				// Manual EWMA calculation for precise benchmarking
				if sender.alpha == 0.0 && markingFraction > 0.0 {
					sender.alpha = 1.0
				} else {
					sender.alpha = (1.0-gain)*sender.alpha + gain*markingFraction
				}

				// Clamp alpha
				if sender.alpha < 0.0 {
					sender.alpha = 0.0
				}
				if sender.alpha > 1.0 {
					sender.alpha = 1.0
				}
			}
		})
	}
}

// BenchmarkCWNDArithmetic benchmarks the arithmetic operations in CWND updates
func BenchmarkCWNDArithmetic(b *testing.B) {
	b.Run("FloatingPoint-Operations", func(b *testing.B) {
		b.ReportAllocs()

		// Simulate Prague's floating point calculations
		alpha := 0.075
		cwnd := float64(32000)
		maxDatagramSize := float64(1200)
		ackedBytes := float64(1200)
		cwndCarry := 0.3

		for b.Loop() {
			// ECN response calculation
			reductionFactor := 1.0 - alpha/2.0
			newCwnd := cwnd * reductionFactor

			// Carry calculation
			carry := cwnd - newCwnd + cwndCarry

			// Additive increase calculation
			unmarkedBytes := ackedBytes * (1.0 - alpha)
			increase := maxDatagramSize * unmarkedBytes / cwnd

			_ = carry
			_ = increase
		}
	})

	b.Run("Integer-Operations", func(b *testing.B) {
		b.ReportAllocs()

		// Simulate RFC9002's integer calculations
		cwnd := protocol.ByteCount(32000)
		maxDatagramSize := protocol.ByteCount(1200)
		ackedBytes := protocol.ByteCount(1200)
		beta := 0.7

		for b.Loop() {
			// Loss response calculation
			newCwnd := protocol.ByteCount(float64(cwnd) * beta)

			// Additive increase calculation
			increase := maxDatagramSize * ackedBytes / cwnd

			_ = newCwnd
			_ = increase
		}
	})
}

// BenchmarkComplexCWNDScenario benchmarks complex CWND update scenarios
func BenchmarkComplexCWNDScenario(b *testing.B) {
	b.Run("Prague-FullUpdate", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()
		sender.l4sEnabled = true
		sender.congestionWindow = 20000

		for b.Loop() {
			iteration := b.Elapsed()

			// Simulate varying conditions
			markingRate := float64((iteration % 20)) / 200.0 // 0% to 10%

			totalBytes := protocol.ByteCount(6000)
			markedBytes := protocol.ByteCount(float64(totalBytes) * markingRate)
			ackedBytes := protocol.ByteCount(1200)

			// Full update cycle
			sender.totalAckedBytes = totalBytes
			sender.ecnMarkedBytes = markedBytes
			sender.updateAlpha()
			sender.applyECNCongestionResponse()
			sender.pragueAdditiveIncrease(ackedBytes)

			// Reset window if it gets too small
			if sender.congestionWindow < 5000 {
				sender.congestionWindow = 20000
			}
		}
	})

	b.Run("RFC9002-FullUpdate", func(b *testing.B) {
		b.ReportAllocs()
		cubicSender := createBenchmarkCubicSender()
		cubicSender.congestionWindow = 20000

		ackedBytes := protocol.ByteCount(1200)
		priorInFlight := protocol.ByteCount(10000)
		eventTime := monotime.Now()

		for b.Loop() {
			iteration := b.Elapsed()

			// Regular packet ack
			cubicSender.OnPacketAcked(protocol.PacketNumber(iteration), ackedBytes, priorInFlight, eventTime)

			// Occasional loss
			if iteration%50 == 0 {
				cubicSender.OnCongestionEvent(protocol.PacketNumber(iteration), ackedBytes, priorInFlight)
			}

			// Reset window if it gets too small
			if cubicSender.congestionWindow < 5000 {
				cubicSender.congestionWindow = 20000
			}
		}
	})
}

// BenchmarkAlphaPrecisionImpact benchmarks the impact of alpha precision
func BenchmarkAlphaPrecisionImpact(b *testing.B) {
	precisionLevels := []struct {
		name     string
		alpha    float64
		decimals int
	}{
		{"Low-2decimals", 0.05, 2},
		{"Medium-3decimals", 0.075, 3},
		{"High-4decimals", 0.0625, 4},
		{"VeryHigh-6decimals", 0.062500, 6},
	}

	for _, level := range precisionLevels {
		b.Run(level.name, func(b *testing.B) {
			b.ReportAllocs()
			sender := createBenchmarkPragueSender()
			sender.alpha = level.alpha
			sender.congestionWindow = 30000

			for b.Loop() {
				sender.applyECNCongestionResponse()
				sender.congestionWindow = 30000 // Reset
			}
		})
	}
}

// BenchmarkMemoryAccessPatterns benchmarks memory access patterns for alpha and CWND
func BenchmarkMemoryAccessPatterns(b *testing.B) {
	b.Run("Prague-MemoryAccess", func(b *testing.B) {
		b.ReportAllocs()
		sender := createBenchmarkPragueSender()

		for b.Loop() {
			// Simulate typical memory access pattern for Prague
			_ = sender.alpha
			_ = sender.alphaGain
			_ = sender.congestionWindow
			_ = sender.cwndCarry
			_ = sender.ecnMarkedBytes
			_ = sender.totalAckedBytes
			_ = sender.l4sEnabled
			_ = sender.inSlowStart
			_ = sender.inRecovery

			// Typical calculation sequence
			markingFraction := float64(sender.ecnMarkedBytes) / float64(sender.totalAckedBytes)
			sender.alpha = (1.0-sender.alphaGain)*sender.alpha + sender.alphaGain*markingFraction
			reductionFactor := 1.0 - sender.alpha/2.0
			newCwnd := float64(sender.congestionWindow) * reductionFactor
			_ = newCwnd
		}
	})

	b.Run("RFC9002-MemoryAccess", func(b *testing.B) {
		b.ReportAllocs()
		cubicSender := createBenchmarkCubicSender()

		for b.Loop() {
			// Simulate typical memory access pattern for RFC9002
			_ = cubicSender.congestionWindow
			_ = cubicSender.slowStartThreshold
			_ = cubicSender.numAckedPackets
			_ = cubicSender.largestAckedPacketNumber
			_ = cubicSender.largestSentPacketNumber
			_ = cubicSender.maxDatagramSize

			// Typical calculation sequence
			packetsInWindow := cubicSender.congestionWindow / cubicSender.maxDatagramSize
			newCwnd := protocol.ByteCount(float64(cubicSender.congestionWindow) * renoBeta)
			_ = packetsInWindow
			_ = newCwnd
		}
	})
}

// Helper function to format benchmark results with comparison
func formatPerformanceComparison(pragueNs, rfc9002Ns float64) string {
	if rfc9002Ns == 0 {
		return "N/A"
	}
	ratio := pragueNs / rfc9002Ns
	if ratio < 1.0 {
		improvement := (1.0 - ratio) * 100
		return fmt.Sprintf("%.1f%% faster", improvement)
	} else {
		overhead := (ratio - 1.0) * 100
		return fmt.Sprintf("%.1f%% slower", overhead)
	}
}
