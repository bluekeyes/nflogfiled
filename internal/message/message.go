package message

import (
	"net"
	"net/netip"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
)

type Message struct {
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

	Packet Packet
}

type Packet struct {
	Src     netip.Addr
	SrcPort uint16
	SrcMAC  net.HardwareAddr

	Dst     netip.Addr
	DstPort uint16
	DstMAC  net.HardwareAddr

	TTL      uint8
	Protocol uint8

	Seq                uint32
	Ack                uint32
	FIN, SYN, RST, ACK bool // TODO(bkeyes): bit flags? boolean fields? drop?
}
