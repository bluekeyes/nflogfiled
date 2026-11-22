package message

import (
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/google/gopacket/layers"
)

type Message struct {
	Fields     FieldSet
	Group      groups.Group
	ReceivedAt time.Time

	Hook   uint8
	Mark   uint32
	Prefix string

	UID uint32
	GID uint32

	InDevice             uint32
	InDeviceName         string
	PhysicalInDevice     uint32
	PhysicalInDeviceName string

	OutDevice             uint32
	OutDeviceName         string
	PhysicalOutDevice     uint32
	PhysicalOutDeviceName string

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
	Length   uint16

	TCPSeq    uint32
	TCPAck    uint32
	TCPFlags  TCPFlag
	TCPWindow uint16
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
