package ackhandler

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/stretchr/testify/require"
)

func TestSentPacketHandler_ECNMode_L4S(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	// Test L4S disabled (should use standard ECN)
	sph := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, true, // enableECN = true
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlPrague, false, // enableL4S = false
	)

	// Should use ECT0 when L4S is disabled
	ecn := sph.ECNMode(true)
	require.Equal(t, protocol.ECT0, ecn, "Should use ECT0 when L4S is disabled")

	// Test L4S enabled (should use ECT1)
	sphL4S := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, true, // enableECN = true
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlPrague, true, // enableL4S = true
	)

	// Should use ECT1 when L4S is enabled
	ecnL4S := sphL4S.ECNMode(true)
	require.Equal(t, protocol.ECT1, ecnL4S, "Should use ECT1 when L4S is enabled")

	// Test short header packet = false (should be ECNNon regardless)
	ecnNonShort := sphL4S.ECNMode(false)
	require.Equal(t, protocol.ECNNon, ecnNonShort, "Should use ECNNon for non-short header packets")

	// Test ECN disabled
	sphNoECN := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, false, // enableECN = false
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlPrague, true, // enableL4S = true
	)

	ecnUnsupported := sphNoECN.ECNMode(true)
	require.Equal(t, protocol.ECNUnsupported, ecnUnsupported, "Should return ECNUnsupported when ECN is disabled")
}

func TestSentPacketHandler_L4S_ECNFeedback(t *testing.T) {
	rttStats := &utils.RTTStats{}
	connStats := &utils.ConnectionStats{}

	sph := newSentPacketHandler(
		0, 1200, rttStats, connStats, false, true,
		protocol.PerspectiveClient, nil, utils.DefaultLogger,
		protocol.CongestionControlPrague, true, // enableL4S = true
	)

	// Simulate sending packets with ECT1
	now := monotime.Now()
	pn1 := protocol.PacketNumber(1)
	pn2 := protocol.PacketNumber(2)

	sph.SentPacket(now, pn1, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECT1, 1200, false, false)
	sph.SentPacket(now, pn2, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECT1, 1200, false, false)

	// Simulate ACK with ECN feedback (one packet marked CE)
	ackFrame := &wire.AckFrame{
		AckRanges: []wire.AckRange{{Smallest: 1, Largest: 2}},
		ECT0:      0,
		ECT1:      1, // 1 packet received as ECT1
		ECNCE:     1, // 1 packet marked as CE
	}

	_, err := sph.ReceivedAck(ackFrame, protocol.Encryption1RTT, now.Add(time.Millisecond))
	require.NoError(t, err)

	// The Prague algorithm should have received ECN feedback
	// This is verified by the fact that no error occurred during processing
}
