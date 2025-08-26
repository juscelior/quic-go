package logging

import (
	"fmt"
	"log"
	"os"
)

// PragueLogger provides debugging output for Prague congestion control and L4S
type PragueLogger struct {
	logger     *log.Logger
	enabled    bool
	connection string // connection identifier for multi-connection debugging
}

// NewPragueLogger creates a new Prague-specific logger
func NewPragueLogger(connectionID string, enabled bool) *PragueLogger {
	return &PragueLogger{
		logger:     log.New(os.Stderr, fmt.Sprintf("[Prague:%s] ", connectionID), log.LstdFlags|log.Lmicroseconds),
		enabled:    enabled,
		connection: connectionID,
	}
}

// LogAlphaUpdate logs alpha parameter updates
func (p *PragueLogger) LogAlphaUpdate(alpha, markingFraction float64, cwnd ByteCount) {
	if !p.enabled {
		return
	}
	p.logger.Printf("Alpha updated: alpha=%.6f marking_fraction=%.6f cwnd=%d", 
		alpha, markingFraction, cwnd)
}

// LogECNFeedback logs ECN feedback reception
func (p *PragueLogger) LogECNFeedback(ecnMarkedBytes, totalBytes ByteCount) {
	if !p.enabled {
		return
	}
	markingRate := float64(ecnMarkedBytes) / float64(totalBytes)
	p.logger.Printf("ECN feedback: marked_bytes=%d total_bytes=%d marking_rate=%.4f", 
		ecnMarkedBytes, totalBytes, markingRate)
}

// LogCongestionWindowChange logs congestion window changes
func (p *PragueLogger) LogCongestionWindowChange(reason string, oldCwnd, newCwnd ByteCount, alpha float64) {
	if !p.enabled {
		return
	}
	change := float64(newCwnd) / float64(oldCwnd)
	p.logger.Printf("Cwnd change (%s): %d -> %d (%.3fx) alpha=%.6f", 
		reason, oldCwnd, newCwnd, change, alpha)
}

// LogL4SState logs L4S state changes
func (p *PragueLogger) LogL4SState(enabled bool, algorithm string) {
	if !p.enabled {
		return
	}
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	p.logger.Printf("L4S %s with algorithm %s", status, algorithm)
}

// LogSlowStartExit logs when slow start is exited
func (p *PragueLogger) LogSlowStartExit(reason string, cwnd ByteCount, alpha float64) {
	if !p.enabled {
		return
	}
	p.logger.Printf("Exited slow start (%s): cwnd=%d alpha=%.6f", reason, cwnd, alpha)
}

// LogPacketLoss logs packet loss events
func (p *PragueLogger) LogPacketLoss(lostBytes ByteCount, cwnd ByteCount) {
	if !p.enabled {
		return
	}
	p.logger.Printf("Packet loss: lost_bytes=%d cwnd=%d", lostBytes, cwnd)
}

// CreateConnectionTracer creates a ConnectionTracer that logs Prague events
func CreatePragueConnectionTracer(connectionID string, enabled bool) *ConnectionTracer {
	if !enabled {
		return nil
	}
	
	logger := NewPragueLogger(connectionID, true)
	
	return &ConnectionTracer{
		UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
			logger.LogAlphaUpdate(alpha, markingFraction, 0) // cwnd not available here
		},
		PragueECNFeedback: func(ecnMarkedBytes ByteCount, totalBytes ByteCount) {
			logger.LogECNFeedback(ecnMarkedBytes, totalBytes)
		},
		L4SStateChanged: func(enabled bool, algorithm string) {
			logger.LogL4SState(enabled, algorithm)
		},
		UpdatedCongestionState: func(state CongestionState) {
			if !enabled {
				return
			}
			logger.logger.Printf("Congestion state: %s", state.String())
		},
	}
}