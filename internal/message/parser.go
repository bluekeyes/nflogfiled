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

			case layers.LayerTypeIPv6:
				msg.Packet.Src, _ = netip.AddrFromSlice(p.ip6Buf.SrcIP)
				msg.Packet.Dst, _ = netip.AddrFromSlice(p.ip6Buf.DstIP)

			case layers.LayerTypeTCP:
				msg.Packet.SrcPort = uint16(p.tcpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.tcpBuf.DstPort)

			case layers.LayerTypeUDP:
				msg.Packet.SrcPort = uint16(p.udpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.udpBuf.DstPort)
			}
		}
	}

	return msg, nil
}
