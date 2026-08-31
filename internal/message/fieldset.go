package message

import (
	"fmt"
	"strings"
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
	FieldPacketLength
	FieldPacketTCPSeq
	FieldPacketTCPAck
	FieldPacketTCPFlags
	FieldPacketTCPWindow

	maxField

	NoFields  FieldSet = 0
	AllFields FieldSet = maxField - 1
)

func (fs FieldSet) Has(f FieldSet) bool {
	return fs&f > 0
}

func (fs *FieldSet) Set(f FieldSet) {
	*fs = *fs | f
}

func (fs FieldSet) String() string {
	return FormatFieldSet(fs, ",")
}

// ParseFieldSet parses a comma-separated list of field names in to a FieldSet.
func ParseFieldSet(s string) (FieldSet, error) {
	var fs FieldSet
	for name := range strings.SplitSeq(s, ",") {
		name = strings.TrimSpace(name)
		field, ok := LookupField(name)
		if !ok {
			return NoFields, fmt.Errorf("unknown field: %q", name)
		}
		fs.Set(field)
	}
	return fs, nil
}

func FormatFieldSet(fs FieldSet, sep string) string {
	// Always print fields in definition order, rather than in random map order.
	var s []string
	for f := FieldSet(1); f <= fs; f <<= 1 {
		if fs.Has(f) {
			if name, ok := namesByField[f]; ok {
				s = append(s, name)
			} else {
				s = append(s, fmt.Sprintf("unknown(%d)", uint64(f)))
			}
		}
	}
	return strings.Join(s, sep)
}

// LookupField returns a FieldSet for the given name, or false if the name does
// not map to a field.
func LookupField(name string) (FieldSet, bool) {
	f, ok := fieldsByName[name]
	return f, ok
}

var fieldsByName map[string]FieldSet
var namesByField map[FieldSet]string

func init() {
	fieldsByName = map[string]FieldSet{
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
		"packet_length":       FieldPacketLength,
		"packet_tcp_seq":      FieldPacketTCPSeq,
		"packet_tcp_ack":      FieldPacketTCPAck,
		"packet_tcp_flags":    FieldPacketTCPFlags,
		"packet_tcp_window":   FieldPacketTCPWindow,
	}

	namesByField = make(map[FieldSet]string, len(fieldsByName))
	for n, f := range fieldsByName {
		namesByField[f] = n
	}
}
