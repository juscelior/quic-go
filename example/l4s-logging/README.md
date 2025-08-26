# L4S Prague Logging Example

This example demonstrates how to enable and use logging for L4S (Low Latency, Low Loss, Scalable Throughput) with Prague congestion control in quic-go.

## Features Demonstrated

- **Prague-specific logging**: Alpha parameter updates, ECN feedback processing
- **L4S state tracking**: Connection initialization and algorithm selection
- **Congestion state monitoring**: Slow start, congestion avoidance, recovery states
- **Performance metrics**: Key metrics for monitoring Prague algorithm behavior

## Running the Example

```bash
go run main.go
```

## Sample Output

```
=== L4S Prague Logging Example ===
L4S Enabled: true
Algorithm: Prague

Configuration is valid!

Logging events that would be generated:
[Prague:demo-conn] 2024/01/15 10:30:15.123456 L4S enabled with algorithm Prague
[Prague:demo-conn] 2024/01/15 10:30:15.123567 Congestion state: SlowStart
[Prague:demo-conn] 2024/01/15 10:30:15.123678 ECN feedback: marked_bytes=1200 total_bytes=4800 marking_rate=0.2500
[Prague:demo-conn] 2024/01/15 10:30:15.123789 Alpha updated: alpha=0.250000 marking_fraction=0.250000 cwnd=0
[Prague:demo-conn] 2024/01/15 10:30:15.123890 Congestion state: CongestionAvoidance
[Prague:demo-conn] 2024/01/15 10:30:15.124001 ECN feedback: marked_bytes=2400 total_bytes=4800 marking_rate=0.5000
[Prague:demo-conn] 2024/01/15 10:30:15.124112 Alpha updated: alpha=0.375000 marking_fraction=0.500000 cwnd=0
```

## Key Events Logged

### L4S State Changes
- **L4SStateChanged**: Logs when L4S is enabled/disabled and which algorithm is used

### Prague Algorithm Events
- **UpdatedPragueAlpha**: Logs alpha parameter updates with marking fraction
- **PragueECNFeedback**: Logs ECN feedback reception with marking rates
- **UpdatedCongestionState**: Standard congestion state transitions

### Configuration
- **Algorithm Selection**: Prague vs RFC9002
- **L4S Enablement**: Whether L4S mode is active
- **ECN Marking**: ECT(1) vs ECT(0) usage

## Using in Real Applications

To enable Prague logging in your application:

```go
// Create Prague-specific tracer
tracer := logging.CreatePragueConnectionTracer("your-conn-id", true)

// Configure QUIC with L4S and logging
config := &quic.Config{
    EnableL4S:                  true,
    CongestionControlAlgorithm: protocol.CongestionControlPrague,
    Tracer:                     tracer,
}

// Use config with quic.Listen or quic.Dial
```

## Metrics for Monitoring

Key metrics to monitor for Prague performance:

- **alpha**: ECN marking fraction (0.0-1.0)
- **marking_fraction**: Instantaneous marking rate
- **congestion_window**: Current congestion window size
- **ecn_marked_bytes**: Bytes marked with CE in last RTT
- **bandwidth_estimate**: Estimated available bandwidth

## Integration with Monitoring Systems

The logging output can be integrated with:

- **Prometheus**: Export metrics via prometheus client
- **InfluxDB**: Time-series data for performance analysis
- **Grafana**: Visualization dashboards
- **Custom monitoring**: Parse log output for specific metrics

## Debugging ECN Issues

Common issues and what to look for:

1. **No ECN feedback**: Check if ECT(1) marking is working
2. **Alpha not updating**: Verify ECN counters in ACK frames
3. **Poor performance**: Monitor marking rates vs alpha values
4. **Fallback to classic**: Check for ECN validation failures