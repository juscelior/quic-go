# L4S Troubleshooting Guide

## Overview

This guide helps diagnose and resolve common issues when using L4S (Low Latency, Low Loss, Scalable Throughput) with Prague congestion control in quic-go.

## Quick Diagnosis Checklist

### ✅ Basic Configuration Check

1. **Verify L4S is enabled correctly:**
```go
config := &quic.Config{
    EnableL4S: true,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
}
```

2. **Check for configuration errors:**
```bash
# This should fail - L4S requires Prague
config := &quic.Config{
    EnableL4S: true,
    CongestionControlAlgorithm: protocol.CongestionControlRFC9002,  // ERROR
}
```

3. **Validate both endpoints support L4S:**
   - Client and server must both use L4S configuration
   - Check that Prague algorithm is selected

## Common Issues and Solutions

### Issue 1: L4S Not Working (No Performance Improvement)

#### Symptoms
- No latency improvement over RFC 9002
- Alpha parameter always zero
- No ECN markings observed

#### Diagnosis Steps

**Step 1: Enable Detailed Logging**
```go
import "log"

tracer := &logging.ConnectionTracer{
    UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
        log.Printf("Alpha updated: %.6f, marking rate: %.4f%%", 
                   alpha, markingFraction*100)
    },
    PragueECNFeedback: func(ecnMarkedBytes, totalBytes protocol.ByteCount) {
        if totalBytes > 0 {
            rate := float64(ecnMarkedBytes) / float64(totalBytes) * 100
            log.Printf("ECN feedback: %d/%d bytes marked (%.2f%%)", 
                       ecnMarkedBytes, totalBytes, rate)
        }
    },
}
```

**Step 2: Check ECN Support**
```bash
# Capture packets to verify ECN markings
sudo tcpdump -i any -v "udp and host <target_ip>" | grep -E "(ECT|CE)"

# Look for:
# - ECT(1) in outgoing packets (Prague marking)
# - CE markings in incoming packets (congestion signals)
```

**Step 3: Verify Network Infrastructure**
```bash
# Test basic ECN capability
ping -Q 1 <target_ip>  # Send ECT(1) marked pings

# Check if network strips ECN bits
traceroute -e <target_ip>
```

#### Common Causes and Solutions

| Cause | Solution |
|-------|----------|
| **Network doesn't support ECN** | Use classic network or deploy L4S-capable infrastructure |
| **Middleboxes strip ECN bits** | Identify and configure middleboxes to preserve ECN |
| **No congestion present** | Test under load or with artificial congestion |
| **AQM not L4S-aware** | Configure L4S-capable AQM (FQ-CoDel, PIE, DualQ) |

### Issue 2: High Latency Despite L4S

#### Symptoms
- Latency higher than expected
- Alpha parameter consistently high (> 0.1)
- Frequent congestion window reductions

#### Diagnosis Steps

**Step 1: Monitor Alpha Evolution**
```go
func monitorAlpha(alpha float64, markingFraction float64) {
    if alpha > 0.1 {
        log.Printf("HIGH ALPHA WARNING: %.4f (marking: %.2f%%)", 
                   alpha, markingFraction*100)
    }
}
```

**Step 2: Check Network Congestion**
```bash
# Monitor network utilization
iftop -i <interface>

# Check queue depths
tc -s qdisc show dev <interface>

# Monitor RTT variation
ping -i 0.1 -c 100 <target_ip> | grep -E "time=|rtt"
```

**Step 3: Analyze ECN Marking Patterns**
```go
// Add to your tracer
var markingHistory []float64

PragueECNFeedback: func(ecnMarkedBytes, totalBytes protocol.ByteCount) {
    if totalBytes > 0 {
        rate := float64(ecnMarkedBytes) / float64(totalBytes)
        markingHistory = append(markingHistory, rate)
        
        // Check for excessive marking
        if rate > 0.2 {
            log.Printf("EXCESSIVE MARKING: %.2f%% - network overloaded", rate*100)
        }
    }
}
```

#### Solutions

| Problem | Solution |
|---------|----------|
| **Network overloaded** | Reduce traffic or upgrade network capacity |
| **AQM too aggressive** | Tune AQM parameters for lower marking threshold |
| **Competing traffic** | Implement proper L4S/classic traffic isolation |
| **Alpha gain too high** | Reduce `pragueAlphaGain` for more stable behavior |

### Issue 3: Configuration Validation Failures

#### Symptoms
- Application crashes on startup
- "L4S can only be enabled when using Prague" error
- Invalid algorithm selection

#### Common Configuration Errors

**Error 1: Wrong Algorithm**
```go
// ❌ WRONG - This will fail validation
config := &quic.Config{
    EnableL4S: true,
    CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
}

// ✅ CORRECT
config := &quic.Config{
    EnableL4S: true,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
}
```

**Error 2: Missing Algorithm Specification**
```go
// ❌ WRONG - EnableL4S without specifying Prague
config := &quic.Config{
    EnableL4S: true,
    // Missing: CongestionControlAlgorithm
}

// ✅ CORRECT
config := &quic.Config{
    EnableL4S: true,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
}
```

### Issue 4: Poor Performance vs RFC 9002

#### Symptoms
- Lower throughput than RFC 9002
- Higher CPU usage
- Memory allocation issues

#### Performance Analysis

**Step 1: Run Benchmarks**
```bash
# Compare algorithm performance
go test -bench="Prague|RFC9002" -benchmem ./internal/congestion/

# Check for performance regressions
go test -bench=BenchmarkPragueVsCubic -benchmem ./internal/congestion/
```

**Step 2: Profile Your Application**
```go
import _ "net/http/pprof"
import "net/http"

// Add pprof endpoint
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Profile with: go tool pprof http://localhost:6060/debug/pprof/profile
```

**Step 3: Monitor Resource Usage**
```bash
# Monitor CPU and memory
top -p $(pgrep your_app)

# Check network interface statistics
cat /proc/net/dev
```

#### Optimization Tips

| Issue | Solution |
|-------|---------|
| **High CPU usage** | Verify ECN processing efficiency, check for tight loops |
| **Memory leaks** | Monitor alpha update frequency, check carry mechanism |
| **Poor throughput** | Tune alpha gain, verify network supports L4S properly |

### Issue 5: ECN Feedback Not Received

#### Symptoms
- Alpha always zero
- No ECN feedback events in logs
- Prague behaves like RFC 9002

#### Diagnosis Steps

**Step 1: Verify ECN Negotiation**
```go
// Check if ECN is negotiated during handshake
SentPacket: func(hdr *logging.Header, size protocol.ByteCount, ecn protocol.ECN, ack *logging.AckFrame, frames []logging.Frame) {
    if ecn == protocol.ECT1 {
        log.Printf("Sent L4S packet with ECT(1)")
    }
}
```

**Step 2: Monitor ACK Frames**
```go
// Check for ECN feedback in ACK frames
ReceivedPacket: func(hdr *logging.Header, size protocol.ByteCount, ecn protocol.ECN, frames []logging.Frame) {
    for _, frame := range frames {
        if ackFrame, ok := frame.(*logging.AckFrame); ok {
            if ackFrame.ECT0 > 0 || ackFrame.ECT1 > 0 || ackFrame.ECNCE > 0 {
                log.Printf("Received ECN feedback: ECT0=%d, ECT1=%d, CE=%d", 
                          ackFrame.ECT0, ackFrame.ECT1, ackFrame.ECNCE)
            }
        }
    }
}
```

**Step 3: Network Path Analysis**
```bash
# Check if ECN bits are preserved
# Send test packets with ECN markings
hping3 -c 10 -i 1 -Q 1 <target_ip>

# Monitor for ECN mangling
tcpdump -i any -v -x "host <target_ip>" | grep -A 5 -B 5 "ECT"
```

#### Solutions

| Problem | Fix |
|---------|-----|
| **ECN negotiation failed** | Ensure both endpoints support AccECN |
| **Middlebox ECN stripping** | Identify and reconfigure network equipment |
| **No congestion to mark** | Test with artificial load or queue delay |

## Debugging Tools and Techniques

### Enable Debug Logging

```go
// Comprehensive Prague debugging
tracer := &logging.ConnectionTracer{
    UpdatedPragueAlpha: func(alpha float64, markingFraction float64) {
        log.Printf("[PRAGUE] Alpha: %.6f, Marking: %.4f%%", alpha, markingFraction*100)
    },
    
    PragueECNFeedback: func(ecnMarkedBytes, totalBytes protocol.ByteCount) {
        if totalBytes > 0 {
            rate := float64(ecnMarkedBytes) / float64(totalBytes)
            log.Printf("[PRAGUE] ECN feedback: %d/%d bytes (%.2f%%)", 
                      ecnMarkedBytes, totalBytes, rate*100)
        }
    },
    
    UpdatedCongestionWindow: func(cwnd protocol.ByteCount) {
        log.Printf("[PRAGUE] CWND updated: %d bytes", cwnd)
    },
    
    SentPacket: func(hdr *logging.Header, size protocol.ByteCount, ecn protocol.ECN, ack *logging.AckFrame, frames []logging.Frame) {
        if ecn == protocol.ECT1 {
            log.Printf("[PRAGUE] Sent L4S packet: ECT(1), size=%d", size)
        }
    },
}
```

### Network Testing Commands

```bash
# Test ECN capability end-to-end
nuttcp -xc -T1 -i1 <target_ip>

# Monitor queue statistics
tc -s qdisc show dev eth0

# Check ECN support in kernel
sysctl net.ipv4.tcp_ecn

# Test with different ECN markings
iperf3 -c <target_ip> --set-mss 1200 --pacing-timer 1000
```

### Performance Baseline Tests

```bash
# Test Prague performance
go run example/l4s-config/main.go -enable-l4s -duration 30s

# Compare with RFC 9002
go run example/l4s-config/main.go -disable-l4s -duration 30s

# Run comprehensive benchmarks
go test -bench=. -benchmem ./internal/congestion/ | tee benchmark_results.txt
```

## Configuration Validation Script

Create a test script to validate your L4S setup:

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/quic-go/quic-go"
    "github.com/quic-go/quic-go/internal/protocol"
    "github.com/quic-go/quic-go/logging"
)

func validateL4SConfig() error {
    // Test 1: Valid L4S configuration
    config := &quic.Config{
        EnableL4S: true,
        CongestionControlAlgorithm: protocol.CongestionControlPrague,
    }
    
    if err := config.Validate(); err != nil {
        return fmt.Errorf("valid L4S config failed validation: %v", err)
    }
    
    // Test 2: Invalid L4S configuration should fail
    invalidConfig := &quic.Config{
        EnableL4S: true,
        CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
    }
    
    if err := invalidConfig.Validate(); err == nil {
        return fmt.Errorf("invalid L4S config passed validation")
    }
    
    log.Println("✅ L4S configuration validation passed")
    return nil
}
```

## Common Network Setup Issues

### AQM Configuration

**FQ-CoDel with L4S support:**
```bash
# Configure dual queue with L4S support
tc qdisc add dev eth0 root fq_codel flows 1024 target 1ms interval 5ms ecn

# Verify configuration
tc qdisc show dev eth0
```

**PIE with ECN:**
```bash
# Enable PIE with ECN marking
tc qdisc add dev eth0 root pie limit 1000 target 15ms ecn

# Monitor PIE statistics
tc -s qdisc show dev eth0
```

### Router Configuration

**Enable ECN support:**
```bash
# Enable ECN in kernel
echo 1 > /proc/sys/net/ipv4/tcp_ecn

# For IPv6
echo 1 > /proc/sys/net/ipv6/conf/all/use_tempaddr
```

## Known Limitations and Workarounds

### Current Limitations

1. **Limited AQM Support**: Not all network equipment supports L4S
2. **ECN Compatibility**: Some middleboxes may strip ECN bits
3. **Performance Overhead**: Small CPU overhead for ECN processing

### Workarounds

1. **Fallback Strategy**: Implement automatic fallback to RFC 9002
```go
config := &quic.Config{
    EnableL4S: false,  // Fallback if L4S fails
    CongestionControlAlgorithm: protocol.CongestionControlRFC9002,
}
```

2. **Gradual Deployment**: Test L4S on specific connections first
3. **Monitoring**: Implement comprehensive monitoring to detect issues

## Performance Validation Checklist

- [ ] Alpha parameter responds to congestion
- [ ] ECN markings are received and processed
- [ ] Latency improvements observed under load
- [ ] Throughput comparable to RFC 9002
- [ ] No memory leaks or excessive CPU usage
- [ ] Proper fallback behavior when L4S unavailable

## Getting Help

If issues persist after following this guide:

1. **Enable comprehensive logging** and collect detailed traces
2. **Run performance benchmarks** to compare with RFC 9002
3. **Check network infrastructure** for L4S compatibility
4. **Review algorithm parameters** and consider tuning
5. **File issues** with detailed logs and network configuration

## Related Documentation

- [Prague Algorithm Tuning Guide](prague-algorithm-tuning.md)
- [L4S Configuration Examples](../example/l4s-config/README.md)
- [IETF L4S Architecture Draft](https://datatracker.ietf.org/doc/draft-ietf-tsvwg-l4s-arch/)

---

*This troubleshooting guide covers common L4S issues in quic-go. For implementation-specific questions, refer to the source code in `/internal/congestion/prague_sender.go` and related test files.*