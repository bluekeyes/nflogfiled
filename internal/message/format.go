package message

import (
	"net/netip"
	"strconv"
	"time"

	"github.com/google/gopacket/layers"
)

type FormatOption func(*formatOptions)

// WithFields sets the output fields for a formatter.
func WithFields(fields FieldSet) FormatOption {
	return func(opts *formatOptions) {
		opts.fields = fields
	}
}

type formatOptions struct {
	fields FieldSet
}

func formatMessage(w writer, msg Message, opts formatOptions) {
	fields := opts.fields

	output := func(f FieldSet) bool {
		return msg.Fields.Has(f) && fields.Has(f)
	}

	w.Open()
	if output(FieldReceivedAt) {
		w.TimeField("timestamp", msg.ReceivedAt)
	}
	if output(FieldGroup) {
		w.UintField("group", uint64(msg.Group))
	}
	if output(FieldHook) {
		w.UintField("hook", uint64(msg.Hook))
	}
	if output(FieldMark) {
		w.UintField("mark", uint64(msg.Mark))
	}
	if output(FieldPrefix) {
		w.QuoteField("prefix", msg.Prefix)
	}
	if output(FieldUID) {
		w.UintField("uid", uint64(msg.UID))
	}
	if output(FieldGID) {
		w.UintField("gid", uint64(msg.GID))
	}
	if output(FieldInDevice) {
		w.StringField("in_dev", FormatDevice(msg.InDeviceName, msg.InDevice))
	}
	if output(FieldPhysicalInDevice) {
		w.StringField("physical_in_dev", FormatDevice(msg.PhysicalInDeviceName, msg.PhysicalInDevice))
	}
	if output(FieldOutDevice) {
		w.StringField("out_dev", FormatDevice(msg.OutDeviceName, msg.OutDevice))
	}
	if output(FieldPhysicalOutDevice) {
		w.StringField("physical_out_dev", FormatDevice(msg.PhysicalOutDeviceName, msg.PhysicalOutDevice))
	}
	if output(FieldVLANIdentifier) {
		w.UintField("vlan_id", uint64(msg.VLANIdentifier))
	}
	if output(FieldPacketProtocol) {
		w.StringField("protocol", FormatProtocol(msg.Packet.Protocol))
	}
	if output(FieldPacketSrcIP) {
		w.AddrField("src_ip", msg.Packet.SrcIP)
	}
	if output(FieldPacketSrcPort) {
		w.UintField("src_port", uint64(msg.Packet.SrcPort))
	}
	if output(FieldPacketSrcMAC) {
		w.StringField("src_mac", msg.Packet.SrcMAC.String())
	}
	if output(FieldPacketDstIP) {
		w.AddrField("dst_ip", msg.Packet.DstIP)
	}
	if output(FieldPacketDstPort) {
		w.UintField("dst_port", uint64(msg.Packet.DstPort))
	}
	if output(FieldPacketDstMAC) {
		w.StringField("dst_mac", msg.Packet.DstMAC.String())
	}
	if output(FieldPacketTTL) {
		w.UintField("packet_ttl", uint64(msg.Packet.TTL))
	}
	if output(FieldPacketLength) {
		w.UintField("length", uint64(msg.Packet.Length))
	}
	if output(FieldPacketTCPSeq) {
		w.UintField("tcp_seq", uint64(msg.Packet.TCPSeq))
	}
	if output(FieldPacketTCPAck) {
		w.UintField("tcp_ack", uint64(msg.Packet.TCPAck))
	}
	if output(FieldPacketTCPFlags) {
		w.StringField("tcp_flags", msg.Packet.TCPFlags.String())
	}
	if output(FieldPacketTCPWindow) {
		w.UintField("tcp_window", uint64(msg.Packet.TCPWindow))
	}

	w.Close()
}

// FormatDevice returns the device name if it is not empty and the device
// index as a string otherwise.
func FormatDevice(name string, index uint32) string {
	if name != "" {
		return name
	}
	return strconv.FormatUint(uint64(index), 10)
}

// FormatProtocol returns the name of common IP protocols or the protocol
// number as a string otherwise.
func FormatProtocol(p layers.IPProtocol) string {
	switch p {
	case layers.IPProtocolTCP:
		return "tcp"
	case layers.IPProtocolUDP:
		return "udp"
	case layers.IPProtocolICMPv4:
		return "icmpv4"
	case layers.IPProtocolICMPv6:
		return "icmpv6"
	default:
		return strconv.FormatUint(uint64(p), 10)
	}
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
