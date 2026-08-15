package testgen

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/google/gopacket/layers"
)

// Observation contains the data observed in an output log line. A nil field
// indicates that the field was not present in the log.
type Observation struct {
	Timestamp *time.Time
	Group     *groups.Group
	Hook      *uint8
	Mark      *uint32
	Prefix    *string
	UID       *uint32
	GID       *uint32
	VLANID    *uint16
	Protocol  *uint8
	SrcIP     *netip.Addr
	SrcPort   *uint16
	SrcMAC    *net.HardwareAddr
	DstIP     *netip.Addr
	DstPort   *uint16
	DstMAC    *net.HardwareAddr
	TTL       *uint8
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
		} else if i := strings.IndexByte(rest, ' '); i >= 0 {
			value, s = rest[:i], rest[i+1:]
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
