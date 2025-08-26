## IETF Draft Analysis

### draft-briscoe-iccrg-prague-congestion-control
- Goals: low queuing delay at high throughput on L4S paths; safe fallback on Classic.
- Packet ID: MUST mark all data with ECT(1) on L4S-capable paths; use AccECN feedback.
- Congestion signals and measurement:
	- Track CE marks via AccECN counters in ACKs; maintain an EWMA alpha of marking fraction.
	- Alpha update (per RTT-equivalent sample):
	  - Let m = CE-marked bytes acknowledged in the sample, a = total bytes acknowledged in the sample (a > 0).
	  - Instantaneous mark fraction f = m / a.
	  - Gain g in [0,1], typically small (e.g., 1/16 .. 1/32).
	  - EWMA: alpha' = (1 - g) * alpha + g * f.
- Congestion response:
	- On ECN: multiplicative decrease proportional to alpha; typical form ssthresh = (1 - alpha/2) * cwnd.
	- Additive increase with reduced RTT-dependence; per-ACK increase accounts for unmarked ACKed bytes.
	  - Example per-ACK increase (bytes): ai = MSS * (1 - alpha) * ackedBytes / cwnd.
	  - Example MD on observed CE in the window: cwnd = max(cwnd_min, cwnd * (1 - alpha/2)).
- On loss: Reno/CUBIC-friendly backoff; combine with ECN if both occur.
- Pacing: pace at roughly inflight/srtt with MTU granularity; avoid extra legacy CUBIC pacing multipliers in CA.
- Startup/restart: fast start with pacing; avoid burstiness; handle app-limited periods cleanly.
- Coexistence: ensure Classic-friendliness on non-L4S paths (detect bleaching/mangling; revert to Not-ECT).
- ECN validity and fallback:
	- Validate that CE/ECT(1) counters evolve consistently (AccECN). If validation fails (bleaching or mangling), disable ECN (send Not-ECT) and run Classic CC.
	- If CE fraction is persistently high (misconfigured AQM), cap MD severity to maintain stability.

### draft-ietf-tsvwg-l4s-arch
- Components: host scalable CC (Prague), network marking/AQM (DualQ-Coupled or FQ variants), protocol identifier ECT(1)/CE.
- Identifier: ECT(1) is the scalable L4S identifier; CE continues to signal congestion at very low queues.
- Coexistence: isolate L4S and Classic traffic (DualQ or FQ) so Classic flows aren’t harmed; sender must fall back when ECN is not usable.
- QUIC fit: QUIC ACK frames already carry ECN counters (ECT0/ECT1/CE); AccECN semantics can be realized natively.
- Deployment: middleboxes may bleach or mangle ECN; sender must validate ECN and disable on failure; privacy and policing considerations apply.

Practical notes for quic-go implementation:
- Use the existing ECN validation state machine; only switch to ECT(1) after validation success.
- Maintain alpha on a per-path basis with RTT-based sampling; avoid bias during app-limited periods.
- Provide pacing rate = cwnd / srtt, with a floor to avoid tiny bursts; integrate with existing pacer.
