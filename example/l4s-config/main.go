package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/internal/protocol"
)

func main() {
	fmt.Println("L4S Configuration Examples")
	
	// Example 1: Valid L4S configuration
	fmt.Println("\n1. Valid L4S Configuration:")
	validL4SConfig := &quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		MaxIdleTimeout:             300 * time.Second,
	}
	
	err := validateConfigExample(validL4SConfig)
	if err != nil {
		fmt.Printf("   ❌ Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Valid configuration\n")
		fmt.Printf("   - EnableL4S: %v\n", validL4SConfig.EnableL4S)
		fmt.Printf("   - Algorithm: %v\n", validL4SConfig.CongestionControlAlgorithm)
	}
	
	// Example 2: Invalid L4S configuration
	fmt.Println("\n2. Invalid L4S Configuration (L4S with RFC9002):")
	invalidL4SConfig := &quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
	}
	
	err = validateConfigExample(invalidL4SConfig)
	if err != nil {
		fmt.Printf("   ❌ Validation failed (as expected): %v\n", err)
	} else {
		fmt.Printf("   ⚠️  Unexpectedly valid\n")
	}
	
	// Example 3: Default configuration
	fmt.Println("\n3. Default Configuration:")
	defaultConfig := &quic.Config{}
	populated := populateConfigExample(defaultConfig)
	
	fmt.Printf("   - EnableL4S: %v\n", populated.EnableL4S)
	fmt.Printf("   - Algorithm: %v\n", populated.CongestionControlAlgorithm)
	
	// Example 4: Prague without L4S (valid)
	fmt.Println("\n4. Prague Algorithm without L4S:")
	pragueConfig := &quic.Config{
		EnableL4S:                  false,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
	}
	
	err = validateConfigExample(pragueConfig)
	if err != nil {
		fmt.Printf("   ❌ Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Valid configuration (Prague can be used without L4S)\n")
	}
	
	// Example 5: Show all available algorithms
	fmt.Println("\n5. Available Congestion Control Algorithms:")
	algorithms := []protocol.CongestionControlAlgorithm{
		protocol.CongestionControlRFC9002,
		protocol.CongestionControlPrague,
	}
	
	for _, alg := range algorithms {
		fmt.Printf("   - %s (%d)\n", alg.String(), alg)
	}
	
	// Example 6: Demonstrate actual client configuration usage
	fmt.Println("\n6. Example Client Configuration for L4S:")
	showClientExample()
}

// validateConfigExample mimics the internal validateConfig function for demonstration
func validateConfigExample(config *quic.Config) error {
	if config == nil {
		return nil
	}
	
	// Validate L4S configuration
	if config.EnableL4S && config.CongestionControlAlgorithm != protocol.CongestionControlPrague {
		return fmt.Errorf("L4S can only be enabled when using Prague congestion control algorithm")
	}
	
	return nil
}

// populateConfigExample mimics the internal populateConfig function for demonstration  
func populateConfigExample(config *quic.Config) *quic.Config {
	if config == nil {
		config = &quic.Config{}
	}
	
	// Set default congestion control algorithm
	algorithm := protocol.CongestionControlRFC9002
	if config.EnableL4S {
		algorithm = protocol.CongestionControlPrague
	} else if config.CongestionControlAlgorithm == protocol.CongestionControlPrague {
		algorithm = config.CongestionControlAlgorithm
	}
	
	return &quic.Config{
		EnableL4S:                  config.EnableL4S,
		CongestionControlAlgorithm: algorithm,
		MaxIdleTimeout:             300 * time.Second, // Default
		HandshakeIdleTimeout:       5 * time.Second,   // Default
	}
}

func showClientExample() {
	fmt.Printf("   ```go\n")
	fmt.Printf("   config := &quic.Config{\n")
	fmt.Printf("       EnableL4S:                  true,\n")
	fmt.Printf("       CongestionControlAlgorithm: protocol.CongestionControlPrague,\n")
	fmt.Printf("       MaxIdleTimeout:             300 * time.Second,\n")
	fmt.Printf("       KeepAlivePeriod:           100 * time.Second,\n")
	fmt.Printf("   }\n")
	fmt.Printf("   \n")
	fmt.Printf("   tlsConfig := &tls.Config{\n")
	fmt.Printf("       NextProtos: []string{\"h3\"},\n")
	fmt.Printf("   }\n")
	fmt.Printf("   \n")
	fmt.Printf("   conn, err := quic.DialAddr(\n")
	fmt.Printf("       context.Background(),\n")
	fmt.Printf("       \"example.com:443\",\n")
	fmt.Printf("       tlsConfig,\n")
	fmt.Printf("       config,\n")
	fmt.Printf("   )\n")
	fmt.Printf("   ```\n")
	
	// Show what would happen in practice
	fmt.Printf("\n   When this configuration is used:\n")
	fmt.Printf("   - Connection will attempt L4S mode\n")
	fmt.Printf("   - ECT(1) marking will be used when L4S is detected\n")
	fmt.Printf("   - Prague congestion control will manage the window\n")
	fmt.Printf("   - Falls back to classic behavior if L4S is not supported\n")
}

func demonstrateActualUsage() {
	fmt.Println("\n7. Actual Usage Example (would connect if server available):")
	
	config := &quic.Config{
		EnableL4S:                  true,
		CongestionControlAlgorithm: protocol.CongestionControlPrague,
		MaxIdleTimeout:             300 * time.Second,
	}
	
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Don't use in production
		NextProtos:         []string{"h3"},
	}
	
	fmt.Printf("   Attempting to connect with L4S enabled...\n")
	
	// This would actually try to connect - commenting out for example
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	conn, err := quic.DialAddr(ctx, "localhost:4433", tlsConfig, config)
	if err != nil {
		fmt.Printf("   ⚠️  Connection failed (expected if no server): %v\n", err)
		return
	}
	defer conn.CloseWithError(0, "demo complete")
	
	fmt.Printf("   ✅ Connected successfully with L4S configuration!\n")
	
	// Check connection state
	state := conn.ConnectionState()
	fmt.Printf("   - Version: %v\n", state.Version)
	fmt.Printf("   - TLS Version: %v\n", state.TLS.Version)
}

func init() {
	// Suppress log output for cleaner example
	log.SetOutput(&discardWriter{})
}

type discardWriter struct{}

func (dw *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}