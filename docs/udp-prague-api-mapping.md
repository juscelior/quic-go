## UDP Prague API and Data Structure Mapping

- Core concepts in UDP Prague (reference: L4STeam/udp_prague):
  - datamessage_t (sender->receiver): seqno, timestamp, length, ECN bits, pacing hints.
  - ackmessage_t (receiver->sender): acked ranges, AccECN counters (ECT0/ECT1/CE), timing info.
  - API hooks: PacketReceived, ACKReceived, GetCCInfo/GetACKInfo, ResetCCInfo.

- Mapping to quic-go:
  - ECN marking on send: sent via send path using protocol.ECN value; quic-go supports ECT0/ECT1/CE in oob control (sys_conn_oob.go).
    - Selection hook: internal/ackhandler.SentPacketHandler.ECNMode(isShortHeader) returns ECN for each packet. Today: ECT0 testing/validation per RFC 9000. Prague: prefer ECT(1) once ECN capable on L4S path.
  - ECN feedback on receive: QUIC ACK frames carry ECT0/ECT1/CE counters; available in wire.AckFrame{ECT0, ECT1, ECNCE} and bubbled to ackhandler.
  - Congestion control state: implement a Prague SendAlgorithm to replace or sit alongside Cubic in internal/congestion, using ECN counts and alpha EWMA.
    - Inputs: bytes_acked, bytes_ecn_ce (delta from ECT1/CE), srtt, min_rtt, inflight, app-limited, loss events.
    - Outputs: cwnd, pacing rate (TimeUntilSend / HasPacingBudget), recovery state (UpdatedCongestionState logging).
  - Pacing: quic-go already paces via congestion.HasPacingBudget(now); Prague should compute budget aligned with pacing_rate ≈ max(cwnd,inflight)/srtt.

- Integration points to modify/add:
  1) internal/ackhandler/ecn.go: after ECN validation succeeds, prefer ECT(1) instead of ECT(0) for L4S mode. Provide a policy toggle.
  2) internal/congestion: add prague sender implementing SendAlgorithmWithDebugInfos; wire it in newSentPacketHandler based on Config.
  3) Config surface: add Config.EnableL4S and Config.CongestionController = "prague" to opt-in; default remains current behavior.
  4) Logging: use logging.ECNStateUpdated and UpdatedCongestionState to trace transitions and cwnd/pacing.

- Safety and fallback:
  - Reuse existing ECN validation logic; if failed or counters inconsistent, fall back to Not-ECT and Classic-friendly behavior.
  - Detect mangling/bleaching automatically (already present) and disable L4S.
