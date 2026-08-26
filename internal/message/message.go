package message

import (
	"math"
	"net"
	"net/netip"
	"strings"
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
	FieldPacketTCPSeq
	FieldPacketTCPAck
	FieldPacketTCPFlags

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
	"packet_tcp_seq":      FieldPacketTCPSeq,
	"packet_tcp_ack":      FieldPacketTCPAck,
	"packet_tcp_flags":    FieldPacketTCPFlags,
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

	// TODO(bkeyes): Add fields for:
	//
	// * IP packet length
	// * TCP window
	//
	// Maybe for these from the netfilter output (don't seem that useful right now):
	//
	// * type of service / precedence (sounds legacy)
	// * ipv4 ID field
	// * ipv4 "don't fragment" flag
	// * TCP urgent pointer
	// * TCP options (as hex or decoded)
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

	TCPSeq   uint32
	TCPAck   uint32
	TCPFlags TCPFlag
}

type TCPFlag uint16

const (
	TCPFlagFIN TCPFlag = 1 << iota
	TCPFlagSYN
	TCPFlagRST
	TCPFlagPSH
	TCPFlagACK
	TCPFlagURG
	TCPFlagECE
	TCPFlagCWR
	TCPFlagNS
)

func TCPFlagFromLayer(tcp *layers.TCP) TCPFlag {
	var f TCPFlag
	setIf := func(packetFlag bool, v TCPFlag) {
		if packetFlag {
			f |= v
		}
	}

	setIf(tcp.FIN, TCPFlagFIN)
	setIf(tcp.SYN, TCPFlagSYN)
	setIf(tcp.RST, TCPFlagRST)
	setIf(tcp.PSH, TCPFlagPSH)
	setIf(tcp.ACK, TCPFlagACK)
	setIf(tcp.URG, TCPFlagURG)
	setIf(tcp.ECE, TCPFlagECE)
	setIf(tcp.CWR, TCPFlagCWR)
	setIf(tcp.NS, TCPFlagNS)

	return f
}

func (f TCPFlag) String() string {
	var s []string
	appendIf := func(flag TCPFlag, v string) {
		if f&flag > 0 {
			s = append(s, v)
		}
	}

	appendIf(TCPFlagFIN, "FIN")
	appendIf(TCPFlagSYN, "SYN")
	appendIf(TCPFlagRST, "RST")
	appendIf(TCPFlagPSH, "PSH")
	appendIf(TCPFlagACK, "ACK")
	appendIf(TCPFlagURG, "URG")
	appendIf(TCPFlagECE, "ECE")
	appendIf(TCPFlagCWR, "CWR")
	appendIf(TCPFlagNS, "NS")

	return strings.Join(s, "|")
}

func appendIfFlag(s []string, all, flag TCPFlag, v string) []string {
	if all&flag > 0 {
		return append(s, v)
	}
	return s
}
