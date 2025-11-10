package quic

import (
	"testing"

	"github.com/quic-go/quic-go/internal/protocol"
)

func TestConfigValidation_L4S(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectError   bool
		errorContains string
	}{
		{
			name:        "nil config should be valid",
			config:      nil,
			expectError: false,
		},
		{
			name:        "empty config should be valid",
			config:      &Config{},
			expectError: false,
		},
		{
			name: "L4S enabled with Prague algorithm should be valid",
			config: &Config{
				EnableL4S:                  true,
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			expectError: false,
		},
		{
			name: "L4S enabled with RFC9002 should be invalid",
			config: &Config{
				EnableL4S:                  true,
				CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
			},
			expectError:   true,
			errorContains: "L4S can only be enabled when using Prague congestion control algorithm",
		},
		{
			name: "L4S enabled with default (RFC9002) algorithm should be invalid",
			config: &Config{
				EnableL4S: true,
				// CongestionControlAlgorithm not set (defaults to RFC9002)
			},
			expectError:   true,
			errorContains: "L4S can only be enabled when using Prague congestion control algorithm",
		},
		{
			name: "L4S disabled with Prague should be valid",
			config: &Config{
				EnableL4S:                  false,
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			expectError: false,
		},
		{
			name: "Prague algorithm without L4S should be valid",
			config: &Config{
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestGetCongestionControlAlgorithm(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected protocol.CongestionControlAlgorithm
	}{
		{
			name:     "nil config should default to RFC9002",
			config:   nil,
			expected: protocol.CongestionControlRFC9002,
		},
		{
			name:     "empty config should default to RFC9002",
			config:   &Config{},
			expected: protocol.CongestionControlRFC9002,
		},
		{
			name: "L4S enabled should force Prague",
			config: &Config{
				EnableL4S: true,
			},
			expected: protocol.CongestionControlPrague,
		},
		{
			name: "explicit Prague should be respected",
			config: &Config{
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			expected: protocol.CongestionControlPrague,
		},
		{
			name: "explicit RFC9002 should be respected",
			config: &Config{
				CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
			},
			expected: protocol.CongestionControlRFC9002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCongestionControlAlgorithm(tt.config)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}


// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPopulateConfig_L4S(t *testing.T) {
	tests := []struct {
		name        string
		input       *Config
		expectedL4S bool
	}{
		{
			name:        "nil config should have L4S disabled",
			input:       nil,
			expectedL4S: false,
		},
		{
			name:        "empty config should have L4S disabled",
			input:       &Config{},
			expectedL4S: false,
		},
		{
			name: "explicitly enabled L4S should be preserved",
			input: &Config{
				EnableL4S:                  true,
				CongestionControlAlgorithm: protocol.CongestionControlPrague,
			},
			expectedL4S: true,
		},
		{
			name: "explicitly disabled L4S should be preserved",
			input: &Config{
				EnableL4S: false,
			},
			expectedL4S: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			populated := populateConfig(tt.input)
			if populated.EnableL4S != tt.expectedL4S {
				t.Errorf("expected EnableL4S %v, got %v", tt.expectedL4S, populated.EnableL4S)
			}
		})
	}
}

