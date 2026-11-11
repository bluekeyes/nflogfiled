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

// TimeValue advances by one second on each call to Now(). It starts from an
// arbitrary fixed time, so by default, every TimeValue produces the same
// sequence of times.
type TimeValue struct {
	Time time.Time
}

func NewTimeValue() *TimeValue {
	return &TimeValue{
		Time: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
	}
}

func (v *TimeValue) Now() time.Time {
	t := v.Time
	v.Time = v.Time.Add(time.Second)
	return t
}

// Values creates values to use in generated test cases. Values are derived
// from a Source of arbitrary data or are deterministic. Provided a sequence of
// calls to the Source returns deterministic values, a sequence of calls to
// methods of Values will always return the same values.
type Values struct {
	src  Source
	time *TimeValue
}

func NewValues(src Source) *Values {
	return &Values{
		src:  src,
		time: NewTimeValue(),
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

func (v *Values) Uint64() uint64 {
	return v.src.Uint64()
}

func (v *Values) Bool() bool {
	return int64(v.src.Uint64()) < 0
}

func (v *Values) BoolRatio(x, y int) bool {
	switch {
	case x <= 0 || y <= 0:
		return false
	case x >= y:
		return true
	}
	// Must return false if Uint64() returns 0 for consistency with Bool()
	return v.IntN(y) >= y-x
}

func (v *Values) IntN(n int) int {
	if n == 0 {
		return 0
	}
	return int(v.src.Uint64() % uint64(n))
}

func (v *Values) Duration(n time.Duration) time.Duration {
	if n == 0 {
		return 0
	}
	return time.Duration(v.src.Uint64() % uint64(n))
}

func (v *Values) Bytes(n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i += 8 {
		val := v.src.Uint64()
		for b := 8; b > 0 && (n-i) >= b; b-- {
			out[i+b-1] = byte(val >> uint64(64-8*b))
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
	return v.time.Now()
}

func (v *Values) Pick[T any](vals ...T) T {
	return vals[v.IntN(len(vals))]
}

// PickIndexN returns n indices from a slice with length, selected without
// replacement.
func (v *Values) PickIndexN(n, length int) []int {
	n = min(n, length)

	out := make([]int, length)
	for i := range length {
		out[i] = i
	}

	// Variant of Fischer-Yates shuffle
	for i := 0; i < n; i++ {
		j := i + v.IntN(length-i)
		out[i], out[j] = out[j], out[i]
	}
	return out[:n]
}
