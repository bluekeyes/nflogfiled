package message

import (
	"bytes"
	"fmt"
	"net"
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

	p.ethParser.IgnoreUnsupported = true
	p.ip4Parser.IgnoreUnsupported = true
	p.ip6Parser.IgnoreUnsupported = true

	return &p
}

func (p *Parser) FromAttribute(group groups.Group, receivedAt time.Time, attr *nflog.Attribute) (Message, error) {
	msg := Message{
		Fields:     FieldGroup | FieldReceivedAt,
		Group:      group,
		ReceivedAt: receivedAt,
	}

	setIfNotNil(&msg, &msg.Hook, attr.Hook, FieldHook)
	setIfNotNil(&msg, &msg.Mark, attr.Mark, FieldMark)
	setIfNotNil(&msg, &msg.Prefix, attr.Prefix, FieldPrefix)
	setIfNotNil(&msg, &msg.UID, attr.UID, FieldUID)
	setIfNotNil(&msg, &msg.GID, attr.GID, FieldGID)

	setIfNotNil(&msg, &msg.InDevice, attr.InDev, FieldInDevice)
	setIfNotNil(&msg, &msg.PhysicalInDevice, attr.PhysInDev, FieldPhysicalInDevice)

	setIfNotNil(&msg, &msg.OutDevice, attr.OutDev, FieldOutDevice)
	setIfNotNil(&msg, &msg.PhysicalOutDevice, attr.PhysOutDev, FieldPhysicalOutDevice)

	if attr.VLAN != nil {
		msg.VLANProtocol = attr.VLAN.Proto
		msg.VLANIdentifier = attr.VLAN.TCI & 0x0FFF
		msg.Fields.Set(FieldVLANProtocol | FieldVLANIdentifier)
	}

	if attr.HwType != nil && attr.HwHeader != nil && *attr.HwType == uint16(layers.LinkTypeEthernet) {
		if err := p.ethBuf.DecodeFromBytes(*attr.HwHeader, gopacket.NilDecodeFeedback); err != nil {
			return Message{}, fmt.Errorf("invalid packet: %w", err)
		}
		msg.Packet.SrcMAC = p.ethBuf.SrcMAC
		msg.Packet.DstMAC = p.ethBuf.DstMAC
		msg.Fields.Set(FieldPacketSrcMAC | FieldPacketDstMAC)
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
			case layers.LayerTypeEthernet:
				msg.Packet.SrcMAC = net.HardwareAddr(bytes.Clone([]byte(p.ethBuf.SrcMAC)))
				msg.Packet.DstMAC = net.HardwareAddr(bytes.Clone([]byte(p.ethBuf.DstMAC)))
				msg.Fields.Set(FieldPacketSrcMAC | FieldPacketDstMAC)

			case layers.LayerTypeIPv4:
				msg.Packet.SrcIP, _ = netip.AddrFromSlice(p.ip4Buf.SrcIP)
				msg.Packet.DstIP, _ = netip.AddrFromSlice(p.ip4Buf.DstIP)
				msg.Packet.TTL = p.ip4Buf.TTL
				msg.Packet.Protocol = p.ip4Buf.Protocol
				msg.Packet.Length = p.ip4Buf.Length
				msg.Fields.Set(FieldPacketSrcIP | FieldPacketDstIP | FieldPacketTTL | FieldPacketProtocol | FieldPacketLength)

			case layers.LayerTypeIPv6:
				msg.Packet.SrcIP, _ = netip.AddrFromSlice(p.ip6Buf.SrcIP)
				msg.Packet.DstIP, _ = netip.AddrFromSlice(p.ip6Buf.DstIP)
				msg.Packet.TTL = p.ip6Buf.HopLimit
				msg.Packet.Protocol = p.ip6Buf.NextHeader
				msg.Packet.Length = p.ip6Buf.Length
				msg.Fields.Set(FieldPacketSrcIP | FieldPacketDstIP | FieldPacketTTL | FieldPacketProtocol | FieldPacketLength)

			case layers.LayerTypeTCP:
				msg.Packet.SrcPort = uint16(p.tcpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.tcpBuf.DstPort)
				msg.Fields.Set(FieldPacketSrcPort | FieldPacketDstPort)

				msg.Packet.TCPSeq = p.tcpBuf.Seq
				msg.Packet.TCPAck = p.tcpBuf.Ack
				msg.Packet.TCPFlags = TCPFlagFromLayer(&p.tcpBuf)
				msg.Packet.TCPWindow = p.tcpBuf.Window
				msg.Fields.Set(FieldPacketTCPSeq | FieldPacketTCPAck | FieldPacketTCPFlags | FieldPacketTCPWindow)

			case layers.LayerTypeUDP:
				msg.Packet.SrcPort = uint16(p.udpBuf.SrcPort)
				msg.Packet.DstPort = uint16(p.udpBuf.DstPort)
				msg.Fields.Set(FieldPacketSrcPort | FieldPacketDstPort)
			}
		}
	}

	return msg, nil
}

func setIfNotNil[T any](msg *Message, dst *T, src *T, f FieldSet) {
	if src != nil {
		*dst = *src
		msg.Fields.Set(f)
	}
}
