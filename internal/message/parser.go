package message

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/florianl/go-nflog/v2"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type Parser struct {
	ethBuf layers.Ethernet
	ip4Buf layers.IPv4
	ip6Buf layers.IPv6
	tcpBuf layers.TCP
	udpBuf layers.UDP

	ethParser *gopacket.DecodingLayerParser
	ip4Parser *gopacket.DecodingLayerParser
	ip6Parser *gopacket.DecodingLayerParser
	decoded   []gopacket.LayerType
}

func NewParser() *Parser {
	p := Parser{
		decoded: []gopacket.LayerType{},
	}

	// Depending on when the kernel generates the nflog message, the payload
	// packet may start at different layers. To avoid errors, construct
	// multiple parsers with different first layers and select the right one
	// based on the received message.
	p.ethParser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&p.ethBuf, &p.ip4Buf, &p.ip6Buf, &p.tcpBuf, &p.udpBuf,
	)
	p.ip4Parser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeIPv4,
		&p.ip4Buf, &p.tcpBuf, &p.udpBuf,
	)
	p.ip6Parser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeIPv6,
		&p.ip6Buf, &p.tcpBuf, &p.udpBuf,
	)

	return &p
}

func (p *Parser) FromAttribute(group groups.Group, receivedAt time.Time, attr *nflog.Attribute) (Message, error) {
	msg := Message{
		Group:      group,
		ReceivedAt: receivedAt,
	}

	setIfNotNil(&msg.Hook, attr.Hook)
	setIfNotNil(&msg.Mark, attr.Mark)
	setIfNotNil(&msg.Prefix, attr.Prefix)
	setIfNotNil(&msg.UID, attr.UID)
	setIfNotNil(&msg.GID, attr.GID)

	setIfNotNil(&msg.InDevice, attr.InDev)
	setIfNotNil(&msg.PhysicalInDevice, attr.PhysInDev)

	setIfNotNil(&msg.OutDevice, attr.OutDev)
	setIfNotNil(&msg.PhysicalOutDevice, attr.PhysOutDev)

	if attr.VLAN != nil {
		msg.VLANProtocol = attr.VLAN.Proto
		msg.VLANIdentifier = attr.VLAN.TCI & 0x0FFF
	}

	if attr.HwType != nil && *attr.HwType == uint16(layers.LinkTypeEthernet) {
		if err := p.ethBuf.DecodeFromBytes(*attr.HwHeader, gopacket.NilDecodeFeedback); err != nil {
			return Message{}, fmt.Errorf("invalid packet: %w", err)
		}
		msg.Packet.SrcMAC = p.ethBuf.SrcMAC
		msg.Packet.DstMAC = p.ethBuf.DstMAC
	}

	// Default to assuming the payload packet is a full ethernet packet unless
	// the message attributes suggest otherwise
	packetParser := p.ethParser
	if attr.HwProtocol != nil {
		switch layers.EthernetType(*attr.HwProtocol) {
		case layers.EthernetTypeIPv4:
			packetParser = p.ip4Parser
		case layers.EthernetTypeIPv6:
			packetParser = p.ip6Parser
		}
	}

	if attr.Payload != nil {
		if err := packetParser.DecodeLayers(*attr.Payload, &p.decoded); err != nil {
			return Message{}, fmt.Errorf("invalid packet: %w", err)
		}
		for _, layerType := range p.decoded {
			switch layerType {
			case layers.LayerTypeIPv4:
				msg.Packet.SrcIP, _ = netip.AddrFromSlice(p.ip4Buf.SrcIP)
				msg.Packet.DstIP, _ = netip.AddrFromSlice(p.ip4Buf.DstIP)
				msg.Packet.TTL = p.ip4Buf.TTL
				msg.Packet.Protocol = uint8(p.ip4Buf.Protocol)

			case layers.LayerTypeIPv6:
				msg.Packet.SrcIP, _ = netip.AddrFromSlice(p.ip6Buf.SrcIP)
				msg.Packet.DstIP, _ = netip.AddrFromSlice(p.ip6Buf.DstIP)
				msg.Packet.TTL = p.ip6Buf.HopLimit
				msg.Packet.Protocol = uint8(p.ip6Buf.NextHeader)

			case layers.LayerTypeTCP:
				msg.Packet.SrcPort = uint16(p.tcpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.tcpBuf.DstPort)
				msg.Packet.Seq = p.tcpBuf.Seq
				msg.Packet.Ack = p.tcpBuf.Ack
				msg.Packet.FIN = p.tcpBuf.FIN
				msg.Packet.SYN = p.tcpBuf.SYN
				msg.Packet.RST = p.tcpBuf.RST
				msg.Packet.ACK = p.tcpBuf.ACK

			case layers.LayerTypeUDP:
				msg.Packet.SrcPort = uint16(p.udpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.udpBuf.DstPort)
			}
		}
	}

	return msg, nil
}

func setIfNotNil[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}
