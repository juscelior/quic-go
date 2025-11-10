package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// L4SMetrics holds all L4S/Prague related Prometheus metrics
type L4SMetrics struct {
	// Prague algorithm metrics
	Alpha            prometheus.Gauge
	MarkingFraction  prometheus.Gauge
	CongestionWindow prometheus.Gauge
	SlowStartActive  prometheus.Gauge
	RecoveryActive   prometheus.Gauge

	// ECN metrics
	ECNMarkedBytesLastRTT prometheus.Gauge
	TotalBytesLastRTT     prometheus.Gauge
	ECNMarkingRate        prometheus.Gauge

	// Connection state
	L4SEnabled prometheus.Gauge
	Algorithm  prometheus.Gauge

	// Performance metrics
	BandwidthEstimate prometheus.Gauge
	RTTMilliseconds   prometheus.Gauge
}

// NewL4SMetrics creates and registers all L4S metrics
func NewL4SMetrics() *L4SMetrics {
	metrics := &L4SMetrics{
		Alpha: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_alpha",
			Help: "Current Prague alpha parameter (ECN marking fraction)",
		}),
		MarkingFraction: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_marking_fraction",
			Help: "Current instantaneous marking fraction",
		}),
		CongestionWindow: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_congestion_window_bytes",
			Help: "Current congestion window in bytes",
		}),
		SlowStartActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_slow_start_active",
			Help: "Whether slow start is currently active (1 = active, 0 = inactive)",
		}),
		RecoveryActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_recovery_active",
			Help: "Whether recovery is currently active (1 = active, 0 = inactive)",
		}),
		ECNMarkedBytesLastRTT: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_ecn_marked_bytes_last_rtt",
			Help: "Bytes marked with CE in the last RTT",
		}),
		TotalBytesLastRTT: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_total_bytes_last_rtt",
			Help: "Total bytes sent in the last RTT",
		}),
		ECNMarkingRate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_ecn_marking_rate",
			Help: "Current ECN marking rate (marked/total)",
		}),
		L4SEnabled: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_l4s_enabled",
			Help: "Whether L4S is enabled (1 = enabled, 0 = disabled)",
		}),
		Algorithm: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_congestion_control_algorithm",
			Help: "Congestion control algorithm (0 = RFC9002, 1 = Prague)",
		}),
		BandwidthEstimate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_bandwidth_estimate_mbps",
			Help: "Estimated available bandwidth in Mbps",
		}),
		RTTMilliseconds: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "quicgo_prague_rtt_milliseconds",
			Help: "Current RTT in milliseconds",
		}),
	}

	// Initialize with default values
	metrics.L4SEnabled.Set(0)
	metrics.Algorithm.Set(0) // RFC9002 by default

	return metrics
}

// UpdateAlpha updates the alpha parameter and related metrics
func (m *L4SMetrics) UpdateAlpha(alpha, markingFraction float64) {
	m.Alpha.Set(alpha)
	m.MarkingFraction.Set(markingFraction)
}

// UpdateCongestionWindow updates the congestion window
func (m *L4SMetrics) UpdateCongestionWindow(cwndBytes float64) {
	m.CongestionWindow.Set(cwndBytes)
}

// UpdateCongestionState updates slow start and recovery states
func (m *L4SMetrics) UpdateCongestionState(slowStart, recovery bool) {
	if slowStart {
		m.SlowStartActive.Set(1)
	} else {
		m.SlowStartActive.Set(0)
	}

	if recovery {
		m.RecoveryActive.Set(1)
	} else {
		m.RecoveryActive.Set(0)
	}
}

// UpdateECNFeedback updates ECN-related metrics
func (m *L4SMetrics) UpdateECNFeedback(markedBytes, totalBytes float64) {
	m.ECNMarkedBytesLastRTT.Set(markedBytes)
	m.TotalBytesLastRTT.Set(totalBytes)

	if totalBytes > 0 {
		markingRate := markedBytes / totalBytes
		m.ECNMarkingRate.Set(markingRate)
	}
}

// UpdateL4SState updates L4S enablement and algorithm
func (m *L4SMetrics) UpdateL4SState(enabled bool, algorithm string) {
	if enabled {
		m.L4SEnabled.Set(1)
		if algorithm == "Prague" {
			m.Algorithm.Set(1)
		} else {
			m.Algorithm.Set(0)
		}
	} else {
		m.L4SEnabled.Set(0)
		m.Algorithm.Set(0)
	}
}

// UpdatePerformanceMetrics updates bandwidth and RTT estimates
func (m *L4SMetrics) UpdatePerformanceMetrics(bandwidthMbps, rttMs float64) {
	m.BandwidthEstimate.Set(bandwidthMbps)
	m.RTTMilliseconds.Set(rttMs)
}
