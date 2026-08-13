package testgen

import (
	"net/netip"
	"time"

	"github.com/bluekeyes/nflogfiled/internal/groups"
)

// Observation contains the data observed in an output log line. A nil field
// indicates that the field was not present in the log.
type Observation struct {
	Timestamp *time.Time
	Group     *groups.Group
	Prefix    *string
	VLANID    *uint16
	Protocol  *uint8
	SrcIP     *netip.Addr
	SrcPort   *uint16
	DstIP     *netip.Addr
	DstPort   *uint16
	TTL       *uint8
}
