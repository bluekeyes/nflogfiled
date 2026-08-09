package message

import (
	"net/netip"
	"strconv"
	"time"
)

type TextFormatter struct {
	Fields FieldSet

	// This is a shared field to avoid allocation on each Format call. The use
	// of the 'writer' interface by 'formatMessage' otherwise causes a local
	// allocation to escape to the heap.
	w textWriter
}

func (tf *TextFormatter) Format(b []byte, msg Message) []byte {
	tf.w.Reset(b)
	formatMessage(&tf.w, msg, tf.Fields)
	return tf.w.b
}

type textWriter struct {
	b           []byte
	hasPrevious bool
}

func (w *textWriter) Reset(b []byte) {
	w.b = b
	w.hasPrevious = false
}

func (w *textWriter) Open() {
}

func (w *textWriter) StringField(name string, value string) {
	b := w.fieldName(name)
	b = append(b, []byte(value)...)
	w.b = b
}

func (w *textWriter) QuoteField(name string, value string) {
	b := w.fieldName(name)
	b = strconv.AppendQuote(b, value)
	w.b = b
}

func (w *textWriter) UintField(name string, value uint64) {
	b := w.fieldName(name)
	b = strconv.AppendUint(b, value, 10)
	w.b = b
}

func (w *textWriter) TimeField(name string, value time.Time) {
	b := w.fieldName(name)
	b, _ = value.AppendText(b)
	w.b = b
}

func (w *textWriter) AddrField(name string, value netip.Addr) {
	b := w.fieldName(name)
	b, _ = value.AppendText(b)
	w.b = b
}

func (w *textWriter) RawField(name string, value []byte) {
	b := w.fieldName(name)
	b = append(b, value...)
	w.b = b
}

func (w *textWriter) fieldName(name string) []byte {
	b := w.b
	if w.hasPrevious {
		b = append(b, ' ')
	} else {
		w.hasPrevious = true
	}
	b = append(b, []byte(name)...)
	b = append(b, '=')
	return b
}

func (w *textWriter) Close() {
	w.b = append(w.b, '\n')
}
