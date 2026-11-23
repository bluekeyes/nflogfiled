package testgen

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"iter"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/gopacket/layers"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

// Observation contains the data observed in an output log line. A nil field
// indicates that the field was not present in the log.
type Observation struct {
	Timestamp  *time.Time
	Group      *groups.Group
	Hook       *uint8
	Mark       *uint32
	Prefix     *string
	UID        *uint32
	GID        *uint32
	InDev      *string
	PhysInDev  *string
	OutDev     *string
	PhysOutDev *string
	VLANID     *uint16
	Protocol   *uint8
	SrcIP      *netip.Addr
	SrcPort    *uint16
	SrcMAC     *net.HardwareAddr
	DstIP      *netip.Addr
	DstPort    *uint16
	DstMAC     *net.HardwareAddr
	TTL        *uint8
	Length     *uint16
	TCPAck     *uint32
	TCPSeq     *uint32
	TCPFlags   *string
	TCPWindow  *uint16
}

// Mask returns a copy of the Observation that contains the intersection of the
// present fields and fields in the FieldSet.
func (o Observation) Mask(fs message.FieldSet) Observation {
	var obs Observation

	if fs.Has(message.FieldReceivedAt) {
		obs.Timestamp = o.Timestamp
	}
	if fs.Has(message.FieldGroup) {
		obs.Group = o.Group
	}
	if fs.Has(message.FieldHook) {
		obs.Hook = o.Hook
	}
	if fs.Has(message.FieldMark) {
		obs.Mark = o.Mark
	}
	if fs.Has(message.FieldPrefix) {
		obs.Prefix = o.Prefix
	}
	if fs.Has(message.FieldUID) {
		obs.UID = o.UID
	}
	if fs.Has(message.FieldGID) {
		obs.GID = o.GID
	}
	if fs.Has(message.FieldInDevice) {
		obs.InDev = o.InDev
	}
	if fs.Has(message.FieldPhysicalInDevice) {
		obs.PhysInDev = o.PhysInDev
	}
	if fs.Has(message.FieldOutDevice) {
		obs.OutDev = o.OutDev
	}
	if fs.Has(message.FieldPhysicalOutDevice) {
		obs.PhysOutDev = o.PhysOutDev
	}
	if fs.Has(message.FieldVLANIdentifier) {
		obs.VLANID = o.VLANID
	}
	if fs.Has(message.FieldPacketProtocol) {
		obs.Protocol = o.Protocol
	}
	if fs.Has(message.FieldPacketSrcIP) {
		obs.SrcIP = o.SrcIP
	}
	if fs.Has(message.FieldPacketSrcPort) {
		obs.SrcPort = o.SrcPort
	}
	if fs.Has(message.FieldPacketSrcMAC) {
		obs.SrcMAC = o.SrcMAC
	}
	if fs.Has(message.FieldPacketDstIP) {
		obs.DstIP = o.DstIP
	}
	if fs.Has(message.FieldPacketDstPort) {
		obs.DstPort = o.DstPort
	}
	if fs.Has(message.FieldPacketDstMAC) {
		obs.DstMAC = o.DstMAC
	}
	if fs.Has(message.FieldPacketTTL) {
		obs.TTL = o.TTL
	}
	if fs.Has(message.FieldPacketLength) {
		obs.Length = o.Length
	}
	if fs.Has(message.FieldPacketTCPAck) {
		obs.TCPAck = o.TCPAck
	}
	if fs.Has(message.FieldPacketTCPSeq) {
		obs.TCPSeq = o.TCPSeq
	}
	if fs.Has(message.FieldPacketTCPFlags) {
		obs.TCPFlags = o.TCPFlags
	}
	if fs.Has(message.FieldPacketTCPWindow) {
		obs.TCPWindow = o.TCPWindow
	}

	return obs
}

func ObservationsFromFile(content []byte, parse func(string) (Observation, error)) iter.Seq2[Observation, error] {
	return func(yield func(Observation, error) bool) {
		for len(content) > 0 {
			var line []byte
			if next := bytes.IndexByte(content, '\n'); next >= 0 {
				line = content[:next]
				content = content[next+1:]
			} else {
				line = content
				content = nil
			}
			if !yield(parse(string(line))) {
				return
			}
		}
	}
}

func DiffObservations(want, got []Observation) string {
	return cmp.Diff(
		want, got,
		cmp.Comparer(func(a, b netip.Addr) bool { return a == b }),
	)
}

// ObservationFromText parses an Observation from a line of text-formatted log output.
func ObservationFromText(line string) (Observation, error) {
	var o Observation
	seen := make(map[string]bool)

	s := strings.TrimSuffix(line, "\n")
	for s != "" {
		name, rest, ok := strings.Cut(s, "=")
		if !ok {
			return o, fmt.Errorf("malformed field near %q", s)
		}

		var value string
		if strings.HasPrefix(rest, `"`) {
			quoted, err := strconv.QuotedPrefix(rest)
			if err != nil {
				return o, fmt.Errorf("field %s: %w", name, err)
			}
			if value, err = strconv.Unquote(quoted); err != nil {
				return o, fmt.Errorf("field %s: %w", name, err)
			}
			s = strings.TrimPrefix(rest[len(quoted):], " ")
		} else if before, after, ok := strings.Cut(rest, " "); ok {
			value, s = before, after
		} else {
			value, s = rest, ""
		}

		if seen[name] {
			return o, fmt.Errorf("field %s appears twice", name)
		}
		seen[name] = true

		if err := setField(&o, name, value); err != nil {
			return o, err
		}
	}

	return o, nil
}

// ObservationFromJSON parses an Observation from a line of JSON-formatted log output.
func ObservationFromJSON(line string) (Observation, error) {
	var o Observation

	var fields map[string]jsontext.Value
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		return o, err
	}

	for name, raw := range fields {
		var value string
		if raw.Kind() == jsontext.KindString {
			if err := json.Unmarshal(raw, &value); err != nil {
				return o, fmt.Errorf("field %s: %w", name, err)
			}
		} else {
			value = string(raw)
		}

		if err := setField(&o, name, value); err != nil {
			return o, err
		}
	}

	return o, nil
}

func setField(o *Observation, name, value string) error {
	switch name {
	case "timestamp":
		var t time.Time
		if err := t.UnmarshalText([]byte(value)); err != nil {
			return fmt.Errorf("field timestamp: %w", err)
		}
		o.Timestamp = &t

	case "group":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field group: %w", err)
		}
		o.Group = new(groups.Group(v))

	case "hook":
		v, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return fmt.Errorf("field hook: %w", err)
		}
		o.Hook = new(uint8(v))

	case "mark":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("field mark: %w", err)
		}
		o.Mark = new(uint32(v))

	case "prefix":
		o.Prefix = &value

	case "uid":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("field uid: %w", err)
		}
		o.UID = new(uint32(v))

	case "gid":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("field gid: %w", err)
		}
		o.GID = new(uint32(v))

	case "in_dev":
		o.InDev = &value

	case "physical_in_dev":
		o.PhysInDev = &value

	case "out_dev":
		o.OutDev = &value

	case "physical_out_dev":
		o.PhysOutDev = &value

	case "vlan_id":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field vlan_id: %w", err)
		}
		o.VLANID = new(uint16(v))

	case "protocol":
		p, err := parseProtocol(value)
		if err != nil {
			return fmt.Errorf("field protocol: %w", err)
		}
		o.Protocol = &p

	case "length":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field length: %w", err)
		}
		o.Length = new(uint16(v))

	case "src_ip":
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("field src_ip: %w", err)
		}
		o.SrcIP = &addr

	case "src_port":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field src_port: %w", err)
		}
		o.SrcPort = new(uint16(v))

	case "src_mac":
		addr, err := net.ParseMAC(value)
		if err != nil {
			return fmt.Errorf("field src_mac: %w", err)
		}
		o.SrcMAC = &addr

	case "dst_ip":
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("field dst_ip: %w", err)
		}
		o.DstIP = &addr

	case "dst_port":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field dst_port: %w", err)
		}
		o.DstPort = new(uint16(v))

	case "dst_mac":
		addr, err := net.ParseMAC(value)
		if err != nil {
			return fmt.Errorf("field dst_mac: %w", err)
		}
		o.DstMAC = &addr

	case "packet_ttl":
		v, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return fmt.Errorf("field packet_ttl: %w", err)
		}
		o.TTL = new(uint8(v))

	case "tcp_seq":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("field tcp_seq: %w", err)
		}
		o.TCPSeq = new(uint32(v))

	case "tcp_ack":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("field tcp_ack: %w", err)
		}
		o.TCPAck = new(uint32(v))

	case "tcp_flags":
		o.TCPFlags = &value

	case "tcp_window":
		v, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("field tcp_window: %w", err)
		}
		o.TCPWindow = new(uint16(v))

	default:
		return fmt.Errorf("unknown field %q", name)
	}

	return nil
}

var protocolNames = map[string]uint8{
	"tcp":    uint8(layers.IPProtocolTCP),
	"udp":    uint8(layers.IPProtocolUDP),
	"icmpv4": uint8(layers.IPProtocolICMPv4),
	"icmpv6": uint8(layers.IPProtocolICMPv6),
}

func parseProtocol(value string) (uint8, error) {
	if p, ok := protocolNames[value]; ok {
		return p, nil
	}
	p, err := strconv.ParseUint(value, 10, 8)
	return uint8(p), err
}
