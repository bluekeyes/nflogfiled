package message

import (
	"net/netip"
	"testing"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
)

func benchMessage() Message {
	return Message{
		Group:      groups.Group(1),
		ReceivedAt: time.Date(2026, 8, 2, 12, 34, 56, 0, time.UTC),
		Packet: Packet{
			SrcIP:   netip.MustParseAddr("192.168.1.1"),
			SrcPort: 54321,
			DstIP:   netip.MustParseAddr("10.0.0.1"),
			DstPort: 443,
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
