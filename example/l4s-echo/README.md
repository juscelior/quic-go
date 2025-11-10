# L4S Echo Example

This example demonstrates the use of L4S (Low Latency, Low Loss, Scalable Throughput) with Prague congestion control in a simple client-server application.

## Features

- **L4S Echo Server**: Server that echoes received data using L4S/Prague
- **L4S Client**: Client that sends data and measures connection performance
- **Performance Monitoring**: Real-time metrics including:
  - Throughput (bytes/second)
  - Latency (RTT)
  - Congestion window size
  - Prague algorithm state

## How to Run

### 1. Start the server (local mode only)

```bash
go run server.go metrics.go
```

### 2. Start the server with Prometheus metrics push

```bash
go run server.go metrics.go -prometheus=http://localhost:5001/prometheus
```

### 3. Start the server with L4S disabled

```bash
go run server.go metrics.go -enable-l4s=false
```

### 4. In another terminal, run the client

```bash
cd client
go run client.go
```

### 5. Run the client with L4S disabled

```bash
cd client
go run client.go -enable-l4s=false
```

## Parameters

- `-enable-l4s`: Enable or disable L4S functionality (default: true)
  - `true`: Enable L4S with Prague congestion control
  - `false`: Use standard RFC9002 congestion control
- `-prometheus`: Optional Prometheus endpoint URL for metrics push
  - If not provided: metrics are displayed only in console/local log
  - If provided: metrics are sent via HTTP POST to the specified endpoint

## L4S Configuration

The example uses conditional L4S configuration based on the `-enable-l4s` flag:

```go
config := &quic.Config{
    EnableL4S:                  *enableL4S,
    CongestionControlAlgorithm: getCongestionControlAlgorithm(*enableL4S),
    MaxIdleTimeout:             300 * time.Second,
}
```

When L4S is enabled, it uses Prague congestion control. When disabled, it falls back to RFC9002.

## Prometheus Metrics

This example exposes comprehensive L4S/Prague metrics to Prometheus. All metrics are prefixed with `quicgo_` and are available as gauges.

### Prague Algorithm Metrics

- **`quicgo_prague_alpha`**: Current Prague alpha parameter (ECN marking fraction)
  - **Type**: Gauge
  - **Description**: The alpha parameter controls how aggressively Prague responds to ECN markings. Values range from 0.0 to 1.0, where higher values indicate more aggressive congestion response.
  - **Use**: Monitor algorithm adaptation to network conditions

- **`quicgo_prague_marking_fraction`**: Current instantaneous marking fraction
  - **Type**: Gauge
  - **Description**: The fraction of packets marked with ECN in the current measurement window. Calculated as marked packets / total packets.
  - **Use**: Track real-time ECN marking intensity

- **`quicgo_prague_congestion_window_bytes`**: Current congestion window in bytes
  - **Type**: Gauge
  - **Description**: The current size of the congestion window, which limits how much data can be in flight.
  - **Use**: Monitor connection throughput capacity

- **`quicgo_prague_slow_start_active`**: Whether slow start is currently active
  - **Type**: Gauge (0 = inactive, 1 = active)
  - **Description**: Indicates if the connection is in the exponential growth phase of slow start.
  - **Use**: Understand connection ramp-up behavior

- **`quicgo_prague_recovery_active`**: Whether recovery is currently active
  - **Type**: Gauge (0 = inactive, 1 = active)
  - **Description**: Indicates if the connection is recovering from packet loss.
  - **Use**: Monitor loss recovery phases

### ECN Metrics

- **`quicgo_prague_ecn_marked_bytes_last_rtt`**: Bytes marked with CE in the last RTT
  - **Type**: Gauge
  - **Description**: Number of bytes that were marked with Congestion Experienced (CE) in the most recent round-trip time.
  - **Use**: Track ECN marking patterns over time

- **`quicgo_prague_total_bytes_last_rtt`**: Total bytes sent in the last RTT
  - **Type**: Gauge
  - **Description**: Total number of bytes transmitted during the last round-trip time period.
  - **Use**: Context for ECN marking rate calculations

- **`quicgo_prague_ecn_marking_rate`**: Current ECN marking rate (marked/total)
  - **Type**: Gauge
  - **Description**: Ratio of ECN-marked bytes to total bytes in the last RTT. Values range from 0.0 to 1.0.
  - **Use**: Monitor network congestion intensity

### Connection State Metrics

- **`quicgo_l4s_enabled`**: Whether L4S is enabled
  - **Type**: Gauge (0 = disabled, 1 = enabled)
  - **Description**: Indicates if L4S functionality is active on this connection.
  - **Use**: Verify configuration and feature activation

- **`quicgo_congestion_control_algorithm`**: Congestion control algorithm
  - **Type**: Gauge (0 = RFC9002, 1 = Prague)
  - **Description**: Which congestion control algorithm is currently being used.
  - **Use**: Confirm algorithm selection based on L4S enablement

### Performance Metrics

- **`quicgo_prague_bandwidth_estimate_mbps`**: Estimated available bandwidth in Mbps
  - **Type**: Gauge
  - **Description**: Current estimate of available network bandwidth in megabits per second.
  - **Use**: Monitor achievable throughput

- **`quicgo_prague_rtt_milliseconds`**: Current RTT in milliseconds
  - **Type**: Gauge
  - **Description**: Current round-trip time measurement in milliseconds.
  - **Use**: Track network latency

## Monitoring

During execution, the client will display real-time metrics:

```
[CLIENT] Connected to server
[CLIENT] Testing with 1024 bytes payload
[CLIENT] ✓ Echo test completed in 0.012s (0.08 MB/s)
[PERF] RTT: 12.5ms | Bytes Sent: 1024 | Packets Sent: 2
[PERF] RTT: 11.8ms | Bytes Sent: 2048 | Packets Sent: 4
...
[CLIENT] All performance tests completed successfully
```

## Troubleshooting

If you encounter issues:

1. Check if the kernel supports ECN: `sysctl net.ipv4.tcp_ecn`
2. Configure L4S-aware AQM: `tc qdisc add dev eth0 root fq_codel ce_threshold 1ms`
3. Use the troubleshooting guide: `../../docs/l4s-troubleshooting.md`

## Files

- `server.go`: L4S echo server
- `client/client.go`: L4S client with monitoring
- `README.md`: This documentation