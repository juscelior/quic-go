package logging

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCongestionState_String(t *testing.T) {
	testCases := []struct {
		state    CongestionState
		expected string
	}{
		{CongestionStateSlowStart, "SlowStart"},
		{CongestionStateCongestionAvoidance, "CongestionAvoidance"},
		{CongestionStateRecovery, "Recovery"},
		{CongestionStateApplicationLimited, "ApplicationLimited"},
		{CongestionState(99), "Unknown"}, // Invalid state
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.state.String()
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestPragueLogger_Creation(t *testing.T) {
	logger := NewPragueLogger("test-conn", true)
	require.NotNil(t, logger)
	require.Equal(t, "test-conn", logger.connection)
	require.True(t, logger.enabled)

	// Test disabled logger
	disabledLogger := NewPragueLogger("disabled-conn", false)
	require.NotNil(t, disabledLogger)
	require.False(t, disabledLogger.enabled)
}

func TestCreatePragueConnectionTracer(t *testing.T) {
	// Test enabled tracer
	tracer := CreatePragueConnectionTracer("test-conn", true)
	require.NotNil(t, tracer)
	require.NotNil(t, tracer.UpdatedPragueAlpha)
	require.NotNil(t, tracer.PragueECNFeedback)
	require.NotNil(t, tracer.L4SStateChanged)
	require.NotNil(t, tracer.UpdatedCongestionState)

	// Test disabled tracer
	disabledTracer := CreatePragueConnectionTracer("disabled", false)
	require.Nil(t, disabledTracer)
}

func TestPragueConnectionTracer_Events(t *testing.T) {
	tracer := CreatePragueConnectionTracer("test-conn", true)
	require.NotNil(t, tracer)

	// Test that calling the tracer functions doesn't panic
	// In a real test, you might want to capture the log output
	
	t.Run("UpdatedPragueAlpha", func(t *testing.T) {
		require.NotPanics(t, func() {
			tracer.UpdatedPragueAlpha(0.25, 0.20)
		})
	})

	t.Run("PragueECNFeedback", func(t *testing.T) {
		require.NotPanics(t, func() {
			tracer.PragueECNFeedback(1200, 4800)
		})
	})

	t.Run("L4SStateChanged", func(t *testing.T) {
		require.NotPanics(t, func() {
			tracer.L4SStateChanged(true, "Prague")
		})
	})

	t.Run("UpdatedCongestionState", func(t *testing.T) {
		require.NotPanics(t, func() {
			tracer.UpdatedCongestionState(CongestionStateSlowStart)
		})
	})
}

// Integration test to verify logging behavior
func TestPragueLogging_Integration(t *testing.T) {
	// This test verifies that the logging components work together
	// without actually producing log output (to keep tests clean)
	
	tracer := CreatePragueConnectionTracer("integration-test", true)
	require.NotNil(t, tracer)
	
	// Simulate a typical sequence of events
	events := []func(){
		func() { tracer.L4SStateChanged(true, "Prague") },
		func() { tracer.UpdatedCongestionState(CongestionStateSlowStart) },
		func() { tracer.PragueECNFeedback(600, 2400) }, // 25% marking
		func() { tracer.UpdatedPragueAlpha(0.25, 0.25) },
		func() { tracer.UpdatedCongestionState(CongestionStateCongestionAvoidance) },
		func() { tracer.PragueECNFeedback(1200, 2400) }, // 50% marking
		func() { tracer.UpdatedPragueAlpha(0.375, 0.50) },
	}
	
	// All events should execute without panicking
	for i, event := range events {
		t.Run(fmt.Sprintf("Event%d", i), func(t *testing.T) {
			require.NotPanics(t, event)
		})
	}
}

func TestPragueLogger_DisabledBehavior(t *testing.T) {
	// Test that disabled loggers don't produce output
	logger := NewPragueLogger("disabled-test", false)
	
	// These calls should be no-ops and not panic
	require.NotPanics(t, func() {
		logger.LogAlphaUpdate(0.5, 0.3, 38400)
		logger.LogECNFeedback(1200, 4800)
		logger.LogCongestionWindowChange("test", 32000, 38400, 0.25)
		logger.LogL4SState(true, "Prague")
		logger.LogSlowStartExit("alpha_threshold", 38400, 0.1)
		logger.LogPacketLoss(1200, 36000)
	})
}