package message

import (
	"math"
	"net"
	"net/netip"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/google/gopacket/layers"
)

type FieldSet uint64

const (
	FieldGroup FieldSet = 1 << iota
	FieldReceivedAt
	FieldHook
	FieldMark
	FieldPrefix
	FieldUID
	FieldGID
	FieldInDevice
	FieldPhysicalInDevice
	FieldOutDevice
	FieldPhysicalOutDevice
	FieldVLANProtocol
	FieldVLANIdentifier
	FieldPacketSrcIP
	FieldPacketSrcPort
	FieldPacketSrcMAC
	FieldPacketDstIP
	FieldPacketDstPort
	FieldPacketDstMAC
	FieldPacketTTL
	FieldPacketProtocol
	FieldPacketSeq
	FieldPacketAck
	FieldPacketFINFlag
	FieldPacketSYNFlag
	FieldPacketRSTFlag
	FieldPacketACKFlag

	// AllFields is the FieldSet containing all fields.
	AllFields FieldSet = math.MaxUint64
)

func (fs FieldSet) Has(f FieldSet) bool {
	return fs&f > 0
}

func (fs *FieldSet) Set(f FieldSet) {
	*fs = *fs | f
}

var FieldsByName = map[string]FieldSet{
	"group":               FieldGroup,
	"received_at":         FieldReceivedAt,
	"hook":                FieldHook,
	"mark":                FieldMark,
	"prefix":              FieldPrefix,
	"uid":                 FieldUID,
	"gid":                 FieldGID,
	"in_device":           FieldInDevice,
	"physical_in_device":  FieldPhysicalInDevice,
	"out_device":          FieldOutDevice,
	"physical_out_device": FieldPhysicalOutDevice,
	"vlan_protocol":       FieldVLANProtocol,
	"vlan_identifier":     FieldVLANIdentifier,
	"packet_src_ip":       FieldPacketSrcIP,
	"packet_src_port":     FieldPacketSrcPort,
	"packet_src_mac":      FieldPacketSrcMAC,
	"packet_dst_ip":       FieldPacketDstIP,
	"packet_dst_port":     FieldPacketDstPort,
	"packet_dst_mac":      FieldPacketDstMAC,
	"packet_ttl":          FieldPacketTTL,
	"packet_protocol":     FieldPacketProtocol,
	"packet_seq":          FieldPacketSeq,
	"packet_ack":          FieldPacketAck,
	"packet_fin_flag":     FieldPacketFINFlag,
	"packet_syn_flag":     FieldPacketSYNFlag,
	"packet_rst_flag":     FieldPacketRSTFlag,
	"packet_ack_flag":     FieldPacketACKFlag,
}

type Message struct {
	Fields     FieldSet
	Group      groups.Group
	ReceivedAt time.Time

	Hook   uint8
	Mark   uint32
	Prefix string

	UID uint32
	GID uint32

	InDevice         uint32
	PhysicalInDevice uint32

	OutDevice         uint32
	PhysicalOutDevice uint32

	VLANProtocol   uint16
	VLANIdentifier uint16

	Packet Packet
}

type Packet struct {
	SrcIP   netip.Addr
	SrcPort uint16
	SrcMAC  net.HardwareAddr

	DstIP   netip.Addr
	DstPort uint16
	DstMAC  net.HardwareAddr

	TTL      uint8
	Protocol layers.IPProtocol

	Seq                uint32
	Ack                uint32
	FIN, SYN, RST, ACK bool // TODO(bkeyes): bit flags? drop?
}
