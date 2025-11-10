package congestion

import (
	"fmt"
	"math"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

const (
	// Prague algorithm constants
	pragueAlphaGain     = 1.0 / 16.0            // EWMA gain for alpha parameter
	pragueMinCwnd       = 2                     // Minimum congestion window in packets
	pragueInitialCwnd   = 32                    // Initial congestion window in packets
	pragueVirtualRTTMin = 25 * time.Millisecond // Minimum virtual RTT for RTT independence
	pragueBeta          = 0.5                   // Classic loss response factor
)

// pragueSender implements the Prague congestion control algorithm for L4S
type pragueSender struct {
	// Core algorithm state
	alpha              float64            // ECN marking fraction EWMA [0,1]
	alphaGain          float64            // EWMA gain for alpha updates
	congestionWindow   protocol.ByteCount // Congestion window in bytes
	slowStartThreshold protocol.ByteCount // Slow start threshold
	cwndCarry          float64            // Fractional cwnd remainder for smooth reductions

	// RTT and timing
	rttStats      *utils.RTTStats
	connStats     *utils.ConnectionStats
	virtualRTTMin time.Duration // Minimum virtual RTT

	// Packet tracking
	largestSentPacketNumber  protocol.PacketNumber
	largestAckedPacketNumber protocol.PacketNumber
	largestSentAtLastCutback protocol.PacketNumber

	// ECN and L4S state
	ecnMarkedBytes  protocol.ByteCount // CE-marked bytes in current RTT
	totalAckedBytes protocol.ByteCount // Total acked bytes in current RTT
	l4sEnabled      bool               // L4S mode enabled

	// Control flags
	inSlowStart                bool
	inRecovery                 bool
	lastCutbackExitedSlowstart bool

	// Configuration
	maxDatagramSize            protocol.ByteCount
	initialCongestionWindow    protocol.ByteCount
	initialMaxCongestionWindow protocol.ByteCount

	// Infrastructure
	pacer *pacer
	clock Clock
}

var (
	_ SendAlgorithm               = &pragueSender{}
	_ SendAlgorithmWithDebugInfos = &pragueSender{}
)

// NewPragueSender creates a new Prague congestion control sender
func NewPragueSender(
	clock Clock,
	rttStats *utils.RTTStats,
	connStats *utils.ConnectionStats,
	initialMaxDatagramSize protocol.ByteCount,
	l4sEnabled bool,
) *pragueSender {
	return newPragueSender(
		clock,
		rttStats,
		connStats,
		initialMaxDatagramSize,
		l4sEnabled,
	)
}

func newPragueSender(
	clock Clock,
	rttStats *utils.RTTStats,
	connStats *utils.ConnectionStats,
	initialMaxDatagramSize protocol.ByteCount,
	l4sEnabled bool,
) *pragueSender {
	p := &pragueSender{
		clock:                      clock,
		rttStats:                   rttStats,
		connStats:                  connStats,
		maxDatagramSize:            initialMaxDatagramSize,
		l4sEnabled:                 l4sEnabled,
		alpha:                      0.0,
		alphaGain:                  pragueAlphaGain,
		virtualRTTMin:              pragueVirtualRTTMin,
		inSlowStart:                true,
		initialCongestionWindow:    protocol.ByteCount(pragueInitialCwnd) * initialMaxDatagramSize,
		initialMaxCongestionWindow: protocol.DefaultInitialMaxStreamData,
	}

	p.congestionWindow = p.initialCongestionWindow
	p.slowStartThreshold = protocol.MaxByteCount
	p.pacer = newPacer(p.BandwidthEstimate)

	return p
}

// SendAlgorithm interface implementation

func (p *pragueSender) TimeUntilSend(bytesInFlight protocol.ByteCount) monotime.Time {
	return p.pacer.TimeUntilSend()
}

func (p *pragueSender) HasPacingBudget(now monotime.Time) bool {
	return p.pacer.Budget(now) >= p.maxDatagramSize
}

func (p *pragueSender) OnPacketSent(
	sentTime monotime.Time,
	bytesInFlight protocol.ByteCount,
	packetNumber protocol.PacketNumber,
	bytes protocol.ByteCount,
	isRetransmittable bool,
) {
	p.pacer.SentPacket(sentTime, bytes)

	if !isRetransmittable {
		return
	}

	if packetNumber > p.largestSentPacketNumber {
		p.largestSentPacketNumber = packetNumber
	}

}

func (p *pragueSender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return bytesInFlight < p.congestionWindow
}

func (p *pragueSender) MaybeExitSlowStart() {
	// Prague exits slow start when ECN marks are detected or when ssthresh is reached
	if p.inSlowStart && (p.alpha > 0 || p.congestionWindow >= p.slowStartThreshold) {
		p.inSlowStart = false
	}
}

func (p *pragueSender) OnPacketAcked(
	number protocol.PacketNumber,
	ackedBytes protocol.ByteCount,
	priorInFlight protocol.ByteCount,
	eventTime monotime.Time,
) {
	if number > p.largestAckedPacketNumber {
		p.largestAckedPacketNumber = number
	}

	// Update total acked bytes for alpha calculation
	p.totalAckedBytes += ackedBytes

	if p.inRecovery && number <= p.largestSentAtLastCutback {
		// Don't increase cwnd during recovery
		return
	}

	if p.inSlowStart {
		p.congestionWindow += ackedBytes
		p.MaybeExitSlowStart()
	} else {
		// Prague additive increase: only for non-ECN marked bytes
		p.pragueAdditiveIncrease(ackedBytes)
	}
}

func (p *pragueSender) OnCongestionEvent(
	number protocol.PacketNumber,
	lostBytes protocol.ByteCount,
	priorInFlight protocol.ByteCount,
) {
	// Prague uses classic loss response (like CUBIC/Reno)
	if number <= p.largestSentAtLastCutback {
		return // Already responded to this loss
	}

	p.lastCutbackExitedSlowstart = p.inSlowStart
	p.inSlowStart = false
	p.inRecovery = true
	p.largestSentAtLastCutback = p.largestSentPacketNumber

	// Classic multiplicative decrease for loss
	p.slowStartThreshold = protocol.ByteCount(float64(p.congestionWindow) * pragueBeta)
	p.congestionWindow = protocol.ByteCount(math.Max(
		float64(p.minCongestionWindow()),
		float64(p.slowStartThreshold),
	))
}

func (p *pragueSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	p.largestSentAtLastCutback = protocol.InvalidPacketNumber
	p.inSlowStart = false
	p.slowStartThreshold = p.congestionWindow / 2
	p.congestionWindow = p.minCongestionWindow()
}

func (p *pragueSender) SetMaxDatagramSize(maxDatagramSize protocol.ByteCount) {
	if maxDatagramSize < p.maxDatagramSize {
		panic(fmt.Sprintf("congestion BUG: decreasing max datagram size from %d to %d", p.maxDatagramSize, maxDatagramSize))
	}
	cwndIsMinCwnd := p.congestionWindow == p.minCongestionWindow()
	p.maxDatagramSize = maxDatagramSize
	if cwndIsMinCwnd {
		p.congestionWindow = p.minCongestionWindow()
	}
}

// SendAlgorithmWithDebugInfos interface implementation

func (p *pragueSender) InSlowStart() bool {
	return p.inSlowStart
}

func (p *pragueSender) InRecovery() bool {
	return p.inRecovery
}

func (p *pragueSender) GetCongestionWindow() protocol.ByteCount {
	return p.congestionWindow
}

// Prague-specific methods

// OnECNFeedback processes ECN feedback and updates alpha parameter
func (p *pragueSender) OnECNFeedback(ecnMarkedBytes protocol.ByteCount) {
	if !p.l4sEnabled {
		return // ECN feedback only relevant in L4S mode
	}

	p.ecnMarkedBytes += ecnMarkedBytes

	// Log ECN feedback for monitoring

	// Update alpha if we have sufficient data (one RTT worth)
	if p.totalAckedBytes > 0 {
		p.updateAlpha()
		p.applyECNCongestionResponse()

		// Reset counters for next RTT
		p.ecnMarkedBytes = 0
		p.totalAckedBytes = 0
	}
}

// updateAlpha updates the ECN marking fraction using EWMA
func (p *pragueSender) updateAlpha() {
	if p.totalAckedBytes == 0 {
		return
	}

	// Calculate instantaneous marking fraction
	markingFraction := float64(p.ecnMarkedBytes) / float64(p.totalAckedBytes)

	// Initialize alpha to 1.0 on first ECN feedback for maximum response
	if p.alpha == 0.0 && markingFraction > 0.0 {
		p.alpha = 1.0
	} else {
		// EWMA update: alpha = (1-g)*alpha + g*f
		p.alpha = (1.0-p.alphaGain)*p.alpha + p.alphaGain*markingFraction
	}

	// Clamp alpha to [0,1]
	if p.alpha < 0.0 {
		p.alpha = 0.0
	}
	if p.alpha > 1.0 {
		p.alpha = 1.0
	}

	// Log alpha updates for debugging and monitoring
}

// applyECNCongestionResponse applies Prague multiplicative decrease based on alpha
func (p *pragueSender) applyECNCongestionResponse() {
	if p.alpha <= 0.0 {
		return
	}

	// Prague multiplicative decrease: cwnd = cwnd * (1 - alpha/2)
	reductionFactor := 1.0 - p.alpha/2.0
	newCwnd := float64(p.congestionWindow) * reductionFactor

	// Track fractional remainder for smoother reductions
	p.cwndCarry += float64(p.congestionWindow) - newCwnd
	if p.cwndCarry >= 1.0 {
		cwndReduction := protocol.ByteCount(p.cwndCarry)
		newCwnd -= float64(cwndReduction)
		p.cwndCarry -= float64(cwndReduction)
	}

	p.congestionWindow = protocol.ByteCount(math.Max(newCwnd, float64(p.minCongestionWindow())))

	// Exit slow start if we're still in it
	if p.inSlowStart {
		p.inSlowStart = false
	}
}

// pragueAdditiveIncrease implements Prague's modified additive increase
func (p *pragueSender) pragueAdditiveIncrease(ackedBytes protocol.ByteCount) {
	if p.congestionWindow >= p.initialMaxCongestionWindow {
		return
	}

	// Prague AI: increase only for non-ECN marked bytes
	// ai = MSS * (1 - alpha) * ackedBytes / cwnd
	unmarkedBytes := ackedBytes // In practice, this would be ackedBytes - ecnMarkedBytes for this ACK
	if p.l4sEnabled && p.alpha > 0 {
		unmarkedBytes = protocol.ByteCount(float64(ackedBytes) * (1.0 - p.alpha))
	}

	increase := float64(p.maxDatagramSize) * float64(unmarkedBytes) / float64(p.congestionWindow)
	p.congestionWindow += protocol.ByteCount(increase)
}

// getVirtualRTT returns virtual RTT for RTT independence
func (p *pragueSender) getVirtualRTT() time.Duration {
	srtt := p.rttStats.SmoothedRTT()
	if srtt < p.virtualRTTMin {
		return p.virtualRTTMin
	}
	return srtt
}

// Helper methods

func (p *pragueSender) minCongestionWindow() protocol.ByteCount {
	return protocol.ByteCount(pragueMinCwnd) * p.maxDatagramSize
}

func (p *pragueSender) BandwidthEstimate() Bandwidth {
	srtt := p.getVirtualRTT()
	if srtt == 0 {
		return Bandwidth(0)
	}
	return BandwidthFromDelta(p.congestionWindow, srtt)
}
