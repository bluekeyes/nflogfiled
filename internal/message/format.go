package message

import (
	"net/netip"
	"time"

	"github.com/google/gopacket/layers"
)

func formatMessage(w writer, msg Message, fields FieldSet) {
	output := func(f FieldSet) bool {
		return msg.Fields.Has(f) && fields.Has(f)
	}

	w.Open()
	if output(FieldGroup) {
		w.UintField("group", uint64(msg.Group))
	}
	if output(FieldReceivedAt) {
		w.TimeField("timestamp", msg.ReceivedAt)
	}
	if output(FieldPrefix) {
		w.QuoteField("prefix", msg.Prefix)
	}
	if output(FieldVLANIdentifier) {
		w.UintField("vlan_id", uint64(msg.VLANIdentifier))
	}
	if output(FieldPacketProtocol) {
		switch msg.Packet.Protocol {
		case layers.IPProtocolTCP:
			w.StringField("protocol", "tcp")
		case layers.IPProtocolUDP:
			w.StringField("protocol", "udp")
		case layers.IPProtocolICMPv4:
			w.StringField("protocol", "icmpv4")
		case layers.IPProtocolICMPv6:
			w.StringField("protocol", "icmpv6")
		default:
			w.UintField("protocol", uint64(msg.Packet.Protocol))
		}
	}
	if output(FieldPacketSrcIP) {
		w.AddrField("src_ip", msg.Packet.SrcIP)
	}
	if output(FieldPacketSrcPort) {
		w.UintField("src_port", uint64(msg.Packet.SrcPort))
	}
	if output(FieldPacketDstIP) {
		w.AddrField("dst_ip", msg.Packet.DstIP)
	}
	if output(FieldPacketDstPort) {
		w.UintField("dst_port", uint64(msg.Packet.DstPort))
	}
	if output(FieldPacketTTL) {
		w.UintField("packet_ttl", uint64(msg.Packet.TTL))
	}
	w.Close()
}

type writer interface {
	Open()
	StringField(string, string)
	QuoteField(string, string)
	UintField(string, uint64)
	TimeField(string, time.Time)
	AddrField(string, netip.Addr)
	RawField(string, []byte)
	Close()
}
