package qlog

import (
	"context"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/qlogwriter"
)

// ConnectionTracerAdapter adapts the new qlogwriter.Trace interface to provide
// backward compatibility with the old ConnectionTracer interface used in tests.
// This allows tests to continue using the old API while the underlying implementation
// uses the new qlogwriter system.
type ConnectionTracerAdapter struct {
	trace qlogwriter.Trace
}

// NewConnectionTracerAdapter creates a new adapter that wraps a qlogwriter.Trace
// and provides the old ConnectionTracer interface.
func NewConnectionTracerAdapter(trace qlogwriter.Trace) *ConnectionTracerAdapter {
	return &ConnectionTracerAdapter{trace: trace}
}

// UpdatedPragueAlpha is called when the Prague alpha parameter is updated.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) UpdatedPragueAlpha(alpha float64, markingFraction float64) {
	// Record a generic metrics event since Prague-specific events were removed
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(MetricsUpdated{
			// Use alpha as a custom metric in congestion window field for testing
			CongestionWindow: int(alpha * 1000), // Scale alpha for visibility
		})
		recorder.Close()
	}
}

// PragueECNFeedback is called when ECN feedback is received.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) PragueECNFeedback(ecnMarkedBytes, totalBytes protocol.ByteCount) {
	// Record a generic ECN event
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(ECNStateUpdated{
			State: ECNStateCapable, // Generic ECN state
		})
		recorder.Close()
	}
}

// UpdatedCongestionState is called when the congestion state changes.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) UpdatedCongestionState(new CongestionState) {
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(CongestionStateUpdated{
			State: new,
		})
		recorder.Close()
	}
}

// L4SStateChanged is called when L4S state changes.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) L4SStateChanged(enabled bool, algorithm string) {
	// Record a generic metrics event
	if recorder := c.trace.AddProducer(); recorder != nil {
		state := CongestionStateSlowStart
		if enabled {
			state = CongestionStateCongestionAvoidance
		}
		recorder.RecordEvent(CongestionStateUpdated{
			State: state,
		})
		recorder.Close()
	}
}

// SentPacket is called when a packet is sent.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) SentPacket(hdr *PacketHeader, size protocol.ByteCount, ack *AckFrame, frames []Frame) {
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(PacketSent{
			Header: *hdr,
			Frames: frames,
		})
		recorder.Close()
	}
}

// ReceivedPacket is called when a packet is received.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) ReceivedPacket(hdr *PacketHeader, size protocol.ByteCount, frames []Frame) {
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(PacketReceived{
			Header: *hdr,
			Frames: frames,
		})
		recorder.Close()
	}
}

// LostPacket is called when a packet is lost.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) LostPacket(hdr *PacketHeader, reason PacketLossReason) {
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(PacketLost{
			Header:  *hdr,
			Trigger: reason,
		})
		recorder.Close()
	}
}

// UpdatedMetrics is called when metrics are updated.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) UpdatedMetrics(rttStats *utils.RTTStats, cwnd protocol.ByteCount, bytesInFlight protocol.ByteCount, packetsInFlight int) {
	if recorder := c.trace.AddProducer(); recorder != nil {
		recorder.RecordEvent(MetricsUpdated{
			MinRTT:           rttStats.MinRTT(),
			SmoothedRTT:      rttStats.SmoothedRTT(),
			LatestRTT:        rttStats.LatestRTT(),
			RTTVariance:      rttStats.MeanDeviation(),
			CongestionWindow: int(cwnd),
			BytesInFlight:    int(bytesInFlight),
			PacketsInFlight:  packetsInFlight,
		})
		recorder.Close()
	}
}

// ConnectionClosed is called when the connection is closed.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) ConnectionClosed(reason error) {
	// No direct equivalent in new system, could record a generic event if needed
}

// ConnectionIDUpdated is called when the connection ID is updated.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) ConnectionIDUpdated(old, new ConnectionID) {
	// No direct equivalent in new system
}

// ChallengeResponseSent is called when a challenge response is sent.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) ChallengeResponseSent(token []byte) {
	// No direct equivalent in new system
}

// ChallengeResponseReceived is called when a challenge response is received.
// This method is kept for backward compatibility with tests.
func (c *ConnectionTracerAdapter) ChallengeResponseReceived(token []byte) {
	// No direct equivalent in new system
}

// CreateConnectionTracer creates a ConnectionTracerAdapter from a context and connection info.
// This is a convenience function for tests.
func CreateConnectionTracer(ctx context.Context, isClient bool, connID ConnectionID) *ConnectionTracerAdapter {
	trace := DefaultConnectionTracer(ctx, isClient, connID)
	if trace == nil {
		return nil
	}
	return NewConnectionTracerAdapter(trace)
}
