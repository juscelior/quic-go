package main

import (
	"context"
	"fmt"
	"log"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/qlog"
)

func main() {
	// Example demonstrating L4S/Prague logging

	// Create Prague-specific logger
	connectionID := "demo-conn"
	pragueTracer := qlog.CreatePragueConnectionTracer(connectionID, true)
	
	// Configure L4S with Prague and logging
	config := &quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		Tracer: func(ctx context.Context, p qlog.Perspective, connID quic.ConnectionID) *qlog.ConnectionTracer {
			return pragueTracer
		},
		KeepAlivePeriod:           0, // Disable keep alive for this example
	}

	// Validate configuration (this would normally be done internally)
	fmt.Println("=== L4S Prague Logging Example ===")
	fmt.Printf("L4S Enabled: %t\n", config.EnableL4S)
	fmt.Printf("Algorithm: %s\n", config.CongestionControlAlgorithm.String())
	
	if config.EnableL4S && config.CongestionControlAlgorithm != protocol.CongestionControlPrague {
		log.Fatal("L4S requires Prague congestion control algorithm")
	}

	fmt.Println("\nConfiguration is valid!")
	fmt.Println("\nLogging events that would be generated:")
	
	// Simulate logging events that would occur during a connection
	simulateLoggingEvents(pragueTracer)
	
	fmt.Println("\n=== Example Complete ===")
	fmt.Println("In a real application, these events would be logged automatically")
	fmt.Println("when using Prague congestion control with L4S enabled.")
}

func simulateLoggingEvents(tracer *qlog.ConnectionTracer) {
	if tracer == nil {
		return
	}
	
	// Simulate connection initialization
	if tracer.L4SStateChanged != nil {
		tracer.L4SStateChanged(true, "Prague")
	}
	
	// Simulate congestion state changes
	if tracer.UpdatedCongestionState != nil {
		tracer.UpdatedCongestionState(qlog.CongestionStateSlowStart)
	}
	
	// Simulate ECN feedback
	if tracer.PragueECNFeedback != nil {
		tracer.PragueECNFeedback(1200, 4800) // 25% marking
	}
	
	// Simulate alpha updates
	if tracer.UpdatedPragueAlpha != nil {
		tracer.UpdatedPragueAlpha(0.25, 0.25) // Alpha updated based on marking
	}
	
	// Simulate congestion avoidance
	if tracer.UpdatedCongestionState != nil {
		tracer.UpdatedCongestionState(qlog.CongestionStateCongestionAvoidance)
	}
	
	// Simulate more ECN feedback with higher marking
	if tracer.PragueECNFeedback != nil {
		tracer.PragueECNFeedback(2400, 4800) // 50% marking
	}
	
	// Simulate alpha increase
	if tracer.UpdatedPragueAlpha != nil {
		tracer.UpdatedPragueAlpha(0.375, 0.50) // Alpha increased
	}
}

// Additional helper functions for extended logging

func logPragueMetrics() {
	fmt.Println("\n=== Prague Metrics Example ===")
	
	// Example metrics that would be useful for Prague monitoring
	metrics := map[string]interface{}{
		"algorithm":                    "Prague",
		"l4s_enabled":                  true,
		"alpha_current":                0.375,
		"marking_fraction_current":     0.50,
		"congestion_window_bytes":      38400,
		"slow_start_active":            false,
		"recovery_active":              false,
		"ecn_marked_bytes_last_rtt":    2400,
		"total_bytes_last_rtt":         4800,
		"rtt_milliseconds":             25.5,
		"bandwidth_estimate_mbps":      12.3,
	}
	
	fmt.Println("Key Prague metrics:")
	for key, value := range metrics {
		fmt.Printf("  %s: %v\n", key, value)
	}
}