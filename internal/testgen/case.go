package testgen

import (
	"net"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/florianl/go-nflog/v2"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Case represents a specific test case with inputs and the expected
// Observation.
type Case struct {
	Group      groups.Group
	ReceivedAt time.Time
	Attr       nflog.Attribute
	Want       Observation
}

var prefixOptions = []string{
	"",
	"input",
	"has space",
	`has "quotes"`,
	"key=value",
	"\t",
	"\n",
	`\`,
	"☃",
	"this_is_a_very_long_topic_with_sixty_three_characters__________",
}

// Generate produces a single random test case.
func Generate(v *Values) Case {
	var attr nflog.Attribute
	var want Observation

	group := groups.Group(v.Uint16())
	want.Group = &group

	receivedAt := v.Time()
	want.Timestamp = &receivedAt

	if v.Bool() {
		hook := v.Uint8()
		attr.Hook = &hook
	}
	if v.Bool() {
		mark := v.Uint32()
		attr.Mark = &mark
	}

	inDev, outDev := v.Uint32(), v.Uint32()
	attr.InDev = &inDev
	attr.OutDev = &outDev

	if v.Bool() {
		physInDev, physOutDev := v.Uint32(), v.Uint32()
		attr.PhysInDev = &physInDev
		attr.PhysOutDev = &physOutDev
	}

	if v.Bool() {
		uid, gid := v.Uint32(), v.Uint32()
		attr.UID = &uid
		attr.GID = &gid
	}

	prefix := Pick(v, prefixOptions...)
	attr.Prefix = &prefix
	want.Prefix = &prefix

	if v.Bool() {
		tci := v.Uint16()
		attr.VLAN = &nflog.VLAN{
			Proto: 0x8100,
			TCI:   tci,
		}
		vlanid := tci & 0xFFF
		want.VLANID = &vlanid
	}

	generatePacket(v, &attr, &want)

	return Case{
		Group:      group,
		ReceivedAt: receivedAt,
		Attr:       attr,
		Want:       want,
	}
}

func generatePacket(v *Values, attr *nflog.Attribute, want *Observation) {
	kind := Pick(v, "ethernet", "ipv4", "ipv6")

	var ipVersion int
	switch kind {
	case "ipv4":
		ipVersion = 4
		hwProtocol := uint16(layers.EthernetTypeIPv4)
		attr.HwProtocol = &hwProtocol
	case "ipv6":
		ipVersion = 6
		hwProtocol := uint16(layers.EthernetTypeIPv6)
		attr.HwProtocol = &hwProtocol
	default:
		ipVersion = Pick(v, 4, 6)
	}

	var protocol layers.IPProtocol
	if ipVersion == 4 {
		protocol = Pick(v, layers.IPProtocolTCP, layers.IPProtocolUDP, layers.IPProtocolICMPv4)
	} else {
		protocol = Pick(v, layers.IPProtocolTCP, layers.IPProtocolUDP, layers.IPProtocolICMPv6)
	}

	ttl := v.Uint8()
	want.TTL = &ttl

	protocolNum := uint8(protocol)
	want.Protocol = &protocolNum

	var ipLayer interface {
		gopacket.NetworkLayer
		gopacket.SerializableLayer
	}
	if ipVersion == 4 {
		srcAddr, dstAddr := v.IPv4Addr(), v.IPv4Addr()
		want.SrcIP, want.DstIP = &srcAddr, &dstAddr

		ipLayer = &layers.IPv4{
			Version:  4,
			TTL:      ttl,
			Protocol: protocol,
			SrcIP:    net.IP(srcAddr.AsSlice()),
			DstIP:    net.IP(dstAddr.AsSlice()),
		}
	} else {
		srcAddr, dstAddr := v.IPv6Addr(), v.IPv4Addr()
		want.SrcIP, want.DstIP = &srcAddr, &dstAddr

		ipLayer = &layers.IPv6{
			Version:    6,
			HopLimit:   ttl,
			NextHeader: protocol,
			SrcIP:      net.IP(srcAddr.AsSlice()),
			DstIP:      net.IP(dstAddr.AsSlice()),
		}
	}

	var protoLayer gopacket.SerializableLayer
	switch protocol {
	case layers.IPProtocolTCP:
		srcPort, dstPort := v.Uint16(), v.Uint16()
		want.SrcPort, want.DstPort = &srcPort, &dstPort

		tcp := &layers.TCP{
			SrcPort: layers.TCPPort(srcPort),
			DstPort: layers.TCPPort(dstPort),
		}
		tcp.SetNetworkLayerForChecksum(ipLayer)
		protoLayer = tcp

	case layers.IPProtocolUDP:
		srcPort, dstPort := v.Uint16(), v.Uint16()
		want.SrcPort, want.DstPort = &srcPort, &dstPort

		udp := &layers.UDP{
			SrcPort: layers.UDPPort(srcPort),
			DstPort: layers.UDPPort(dstPort),
		}
		udp.SetNetworkLayerForChecksum(ipLayer)
		protoLayer = udp

	case layers.IPProtocolICMPv4:
		protoLayer = &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		}

	case layers.IPProtocolICMPv6:
		icmp6 := &layers.ICMPv6{
			TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
		}
		icmp6.SetNetworkLayerForChecksum(ipLayer)
		protoLayer = icmp6
	}

	var ls []gopacket.SerializableLayer
	if kind == "ethernet" {
		ethType := layers.EthernetTypeIPv4
		if ipVersion == 6 {
			ethType = layers.EthernetTypeIPv6
		}
		ls = append(ls, &layers.Ethernet{
			SrcMAC:       net.HardwareAddr(v.Bytes(6)),
			DstMAC:       net.HardwareAddr(v.Bytes(6)),
			EthernetType: ethType,
		})
	}
	ls = append(ls, ipLayer, protoLayer)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, ls...); err != nil {
		panic("testgen: failed to serialize packet: " + err.Error())
	}
	payload := buf.Bytes()
	attr.Payload = &payload
}
