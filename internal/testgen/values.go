package testgen

import (
	"encoding/binary"
	"net/netip"
	"time"
)

// Source is a source of arbitrary data used to generate values. Depending on
// the purpose of the test, the data may or may not be random.
type Source interface {
	Uint64() uint64
}

// ByteSource returns a Source that returns values from the byte array in
// sequence. It returns 0 when it runs out of bytes.
func ByteSource(b []byte) Source {
	return &byteSource{b: b}
}

type byteSource struct {
	b []byte
}

func (s *byteSource) Uint64() uint64 {
	if len(s.b) == 0 {
		return 0
	}
	var v [8]byte
	n := copy(v[:], s.b)
	s.b = s.b[n:]
	return binary.BigEndian.Uint64(v[:])
}

// Values creates values to use in generated test cases. Values are derived
// from a Source of arbitrary data or are deterministic. Provided a sequence of
// calls to the Source returns deterministic values, a sequence of calls to
// methods of Values will always return the same values.
type Values struct {
	src  Source
	time time.Time
}

func NewValues(src Source) *Values {
	return &Values{
		src:  src,
		time: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
	}
}

func (v *Values) Uint8() uint8 {
	return uint8(v.src.Uint64() >> 56)
}

func (v *Values) Uint16() uint16 {
	return uint16(v.src.Uint64() >> 48)
}

func (v *Values) Uint32() uint32 {
	return uint32(v.src.Uint64() >> 32)
}

func (v *Values) Bool() bool {
	return int64(v.src.Uint64()) < 0
}

func (v *Values) IntN(n int) int {
	return int(v.src.Uint64() % uint64(n))
}

func (v *Values) Bytes(n int) []byte {
	out := make([]byte, n)
	for i := 0; n > 0; n -= 8 {
		val := v.src.Uint64()
		for b := 8; b > 0; b-- {
			if n >= b {
				out[i+b-1] = byte(val >> uint64(64-8*b))
			}
		}
	}
	return out
}

func (v *Values) IPv4Addr() netip.Addr {
	addr, _ := netip.AddrFromSlice(v.Bytes(4))
	return addr
}

func (v *Values) IPv6Addr() netip.Addr {
	addr, _ := netip.AddrFromSlice(v.Bytes(16))
	return addr
}

// Time returns a monotonically increasing value. Unlike other methods, the
// time value is not derived from the Source. It starts from a known value and
// each call moves forward by a fixed amount.
func (v *Values) Time() time.Time {
	t := v.time
	v.time = v.time.Add(time.Second)
	return t
}

// TODO(bkeyes): add this to Values once Go 1.27 is out and allows generic methods
func Pick[T any](v *Values, vals ...T) T {
	return vals[v.IntN(len(vals))]
}
