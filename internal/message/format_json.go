package message

import (
	"encoding"
	"strconv"
)

type JSONFormatter struct{}

func (jf JSONFormatter) Format(b []byte, msg Message) []byte {
	w := jsonWriter{b: b}
	formatMessage(&w, msg)
	return w.b
}

type jsonWriter struct {
	b           []byte
	hasPrevious bool
}

// TODO(bkeyes): possible optimization: use slices.Grow in each function to
// guarantee at most one allocation. In practice, because the buffer is
// perpetually reused, allocations should only happen when a new message is
// either longer than the initial buffer size or longer than any previous
// message. Amortized over all messages, it's probably not worth the extra
// complexity of slices.Grow calls, especially if the total size isn't known.

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

func (w *jsonWriter) UintField(name string, value uint64) {
	b := w.fieldName(name)
	b = strconv.AppendUint(b, value, 10)
	w.b = b
}

func (w *jsonWriter) TextField(name string, value encoding.TextAppender) {
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
