package message

import (
	"net/netip"
	"time"
)

func formatMessage(w writer, msg Message) {
	// TODO(bkeyes): only write fields that are present and selected
	w.Open()
	w.UintField("group", uint64(msg.Group))
	w.TimeField("timestamp", msg.ReceivedAt)
	w.AddrField("src_ip", msg.Packet.SrcIP)
	w.UintField("src_port", uint64(msg.Packet.SrcPort))
	w.AddrField("dst_ip", msg.Packet.DstIP)
	w.UintField("src_port", uint64(msg.Packet.DstPort))
	w.Close()
}

type writer interface {
	Open()
	StringField(string, string)
	UintField(string, uint64)
	TimeField(string, time.Time)
	AddrField(string, netip.Addr)
	RawField(string, []byte)
	Close()
}
