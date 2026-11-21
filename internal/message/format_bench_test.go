package message

import (
	"net/netip"
	"testing"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/google/gopacket/layers"
)

func benchMessage() Message {
	return Message{
		Fields:     AllFields,
		Group:      groups.Group(1),
		ReceivedAt: time.Date(2026, 8, 2, 12, 34, 56, 0, time.UTC),
		InDevice:   2,
		OutDevice:  3,
		Packet: Packet{
			SrcIP:    netip.MustParseAddr("192.168.1.1"),
			SrcPort:  54321,
			DstIP:    netip.MustParseAddr("10.0.0.1"),
			DstPort:  443,
			Protocol: layers.IPProtocolTCP,
			TCPSeq:   2,
			TCPAck:   1,
		},
	}
}

func BenchmarkJSONFormatter(b *testing.B) {
	msg := benchMessage()
	buf := make([]byte, 0, 256)
	fmt := NewJSONFormatter()

	b.ResetTimer()
	for b.Loop() {
		buf = fmt.Format(buf, msg)
		buf = buf[:0]
	}
}

func BenchmarkTextFormatter(b *testing.B) {
	msg := benchMessage()
	buf := make([]byte, 0, 256)
	fmt := NewTextFormatter()

	b.ResetTimer()
	for b.Loop() {
		buf = fmt.Format(buf, msg)
		buf = buf[:0]
	}
}
