package message

import (
	"encoding/json/jsontext"
	"net/netip"
	"strconv"
	"time"
)

type JSONFormatter struct {
	Fields FieldSet

	// This is a shared field to avoid allocation on each Format call. The use
	// of the 'writer' interface by 'formatMessage' otherwise causes a local
	// allocation to escape to the heap.
	w jsonWriter
}

func (jf *JSONFormatter) Format(b []byte, msg Message) []byte {
	jf.w.Reset(b)
	formatMessage(&jf.w, msg, jf.Fields)
	return jf.w.b
}

type jsonWriter struct {
	b           []byte
	hasPrevious bool
}

func (w *jsonWriter) Reset(b []byte) {
	w.b = b
	w.hasPrevious = false
}

func (w *jsonWriter) Open() {
	w.b = append(w.b, '{')
}

func (w *jsonWriter) StringField(name string, value string) {
	b := w.fieldName(name)
	b = append(b, '"')
	b = append(b, []byte(value)...)
	b = append(b, '"')
	w.b = b
}

func (w *jsonWriter) QuoteField(name string, value string) {
	b := w.fieldName(name)
	b, _ = jsontext.AppendQuote(b, value)
	w.b = b
}

func (w *jsonWriter) UintField(name string, value uint64) {
	b := w.fieldName(name)
	b = strconv.AppendUint(b, value, 10)
	w.b = b
}

func (w *jsonWriter) TimeField(name string, value time.Time) {
	b := w.fieldName(name)
	b = append(b, '"')
	b, _ = value.AppendText(b)
	b = append(b, '"')
	w.b = b
}

func (w *jsonWriter) AddrField(name string, value netip.Addr) {
	b := w.fieldName(name)
	b = append(b, '"')
	b, _ = value.AppendText(b)
	b = append(b, '"')
	w.b = b
}

func (w *jsonWriter) RawField(name string, value []byte) {
	b := w.fieldName(name)
	b = append(b, value...)
	w.b = b
}

func (w *jsonWriter) fieldName(name string) []byte {
	b := w.b
	if w.hasPrevious {
		b = append(b, ',')
	} else {
		w.hasPrevious = true
	}
	b = append(b, '"')
	b = append(b, []byte(name)...)
	b = append(b, []byte(`":`)...)
	return b
}

func (w *jsonWriter) Close() {
	w.b = append(w.b, []byte("}\n")...)
}
