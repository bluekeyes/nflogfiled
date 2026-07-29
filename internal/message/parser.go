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

	layerParser *gopacket.DecodingLayerParser
	decoded     []gopacket.LayerType
}

func NewParser() *Parser {
	p := Parser{
		decoded: []gopacket.LayerType{},
	}

	p.layerParser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&p.ethBuf, &p.ip4Buf, &p.ip6Buf, &p.tcpBuf, &p.udpBuf,
	)

	return &p
}

func (p *Parser) FromAttribute(group groups.Group, attr *nflog.Attribute) (Message, error) {
	msg := Message{
		Group:      group,
		ReceivedAt: time.Now(),
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

	if attr.Payload != nil {
		if err := p.layerParser.DecodeLayers(*attr.Payload, &p.decoded); err != nil {
			return Message{}, fmt.Errorf("invalid packet: %w", err)
		}
		for _, layerType := range p.decoded {
			switch layerType {
			case layers.LayerTypeEthernet:
				msg.Packet.SrcMAC = p.ethBuf.SrcMAC
				msg.Packet.DstMAC = p.ethBuf.DstMAC

			case layers.LayerTypeIPv4:
				msg.Packet.Src, _ = netip.AddrFromSlice(p.ip4Buf.SrcIP)
				msg.Packet.Dst, _ = netip.AddrFromSlice(p.ip4Buf.DstIP)
				msg.Packet.TTL = p.ip4Buf.TTL
				msg.Packet.Protocol = uint8(p.ip4Buf.Protocol)

			case layers.LayerTypeIPv6:
				msg.Packet.Src, _ = netip.AddrFromSlice(p.ip6Buf.SrcIP)
				msg.Packet.Dst, _ = netip.AddrFromSlice(p.ip6Buf.DstIP)
				msg.Packet.TTL = p.ip6Buf.HopLimit
				msg.Packet.Protocol = uint8(p.ip6Buf.NextHeader)

			case layers.LayerTypeTCP:
				msg.Packet.SrcPort = uint16(p.tcpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.tcpBuf.DstPort)
				msg.Packet.Seq = p.tcpBuf.Seq
				msg.Packet.Ack = p.tcpBuf.Ack

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
