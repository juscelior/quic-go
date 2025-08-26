# L4S Configuration Examples

This directory contains examples of how to configure quic-go to use L4S (Low Latency, Low Loss, Scalable Throughput) with Prague congestion control modifications.

## Basic L4S Configuration

```go
config := &quic.Config{
    EnableL4S: true,
}
```

## Valid Configuration Combinations

### L4S Enabled (Requires Prague)
```go
// ✅ VALID: L4S with Prague algorithm
config := &quic.Config{
    EnableL4S:                  true,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
}
```

### L4S Disabled (RFC9002 or Prague)
```go
// ✅ VALID: Standard QUIC with RFC9002 (default)
config := &quic.Config{
    EnableL4S: false,
    // CongestionControlAlgorithm defaults to RFC9002
}

// ✅ VALID: Explicit RFC9002 without L4S
config := &quic.Config{
    EnableL4S:                  false,
    CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
}

// ✅ VALID: Prague algorithm without L4S (falls back to classic behavior)
config := &quic.Config{
    EnableL4S:                  false,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
}
```

## Invalid Configuration Combinations

```go
// ❌ INVALID: L4S with RFC9002 will fail validation
config := &quic.Config{
    EnableL4S:                  true,
    CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
}
// Error: "L4S can only be enabled when using Prague congestion control algorithm"

// ❌ INVALID: L4S with default (RFC9002) algorithm will fail validation
config := &quic.Config{
    EnableL4S: true,
    // CongestionControlAlgorithm not set (defaults to RFC9002)
}
// Error: "L4S can only be enabled when using Prague congestion control algorithm"
```

## Complete Example with L4S

```go
package main

import (
    "context"
    "crypto/tls"
    
    "github.com/quic-go/quic-go"
    "github.com/quic-go/quic-go/internal/protocol"
)

func main() {
    // Client configuration with L4S enabled
    clientConfig := &quic.Config{
        EnableL4S:                  true,
        CongestionControlAlgorithm: protocol.CongestionControlPrague,
        MaxIdleTimeout:             300 * time.Second,
        KeepAlivePeriod:           100 * time.Second,
    }
    
    // Server configuration with L4S enabled
    serverConfig := &quic.Config{
        EnableL4S:                  true,
        CongestionControlAlgorithm: protocol.CongestionControlPrague,
        MaxIdleTimeout:             300 * time.Second,
    }
    
    // TLS configuration
    tlsConfig := &tls.Config{
        InsecureSkipVerify: true, // Don't do this in production
        NextProtos:         []string{"h3"},
    }
    
    // Dial with L4S configuration
    conn, err := quic.DialAddr(
        context.Background(),
        "localhost:4433",
        tlsConfig,
        clientConfig,
    )
    if err != nil {
        panic(err)
    }
    defer conn.CloseWithError(0, "")
    
    // Use the connection...
}
```

## Configuration Behavior

### Default Behavior (L4S Disabled)
- **EnableL4S**: `false` (default)
- **CongestionControlAlgorithm**: `protocol.CongestionControlRFC9002` (default)
- **ECN Mode**: Classic ECN behavior (ECT(0) when available)
- **Fallback**: Standard QUIC behavior

### L4S Enabled Behavior
- **EnableL4S**: `true` (explicit)
- **CongestionControlAlgorithm**: Must be `protocol.CongestionControlPrague`
- **ECN Mode**: L4S ECN behavior (ECT(1) on capable paths)
- **Fallback**: Automatic fallback to classic behavior when:
  - Network doesn't support L4S
  - ECN validation fails
  - Peer doesn't support L4S

## Network Requirements for L4S

For L4S to be effective, the network path must support:

1. **ECN (Explicit Congestion Notification)**
   - Network elements preserve ECN markings
   - No ECN bleaching or mangling

2. **L4S-Capable AQM (Active Queue Management)**
   - Dual-Queue Coupled AQM or similar
   - Support for ECT(1) traffic identification
   - Low-latency queue for L4S traffic

3. **Low-Latency Infrastructure**
   - Network designed for sub-millisecond queuing delays
   - Appropriate buffer sizing

## Performance Expectations

### With L4S Support
- **Latency**: Sub-millisecond average queuing delay
- **Throughput**: Full link utilization
- **Responsiveness**: Rapid adaptation to congestion
- **Scalability**: Consistent performance across flow rates

### Fallback to Classic
- **Latency**: Standard QUIC latency characteristics
- **Throughput**: CUBIC or Reno performance
- **Compatibility**: Works on all networks
- **Interoperability**: Compatible with existing QUIC implementations