package message

import (
	"encoding"
	"strconv"
)

type TextFormatter struct{}

func (tf TextFormatter) Format(b []byte, msg Message) []byte {
	w := textWriter{b: b}
	formatMessage(&w, msg)
	return w.b
}

type textWriter struct {
	b           []byte
	hasPrevious bool
}

// TODO(bkeyes): possible optimization: use slices.Grow in each function to
// guarantee at most one allocation. In practice, because the buffer is
// perpetually reused, allocations should only happen when a new message is
// either longer than the initial buffer size or longer than any previous
// message. Amortized over all messages, it's probably not worth the extra
// complexity of slices.Grow calls, especially if the total size isn't known.

func (w *textWriter) Open() {
}

func (w *textWriter) StringField(name string, value string) {
	b := w.fieldName(name)
	b = append(b, []byte(value)...)
	w.b = b
}

func (w *textWriter) UintField(name string, value uint64) {
	b := w.fieldName(name)
	b = strconv.AppendUint(b, value, 10)
	w.b = b
}

func (w *textWriter) TextField(name string, value encoding.TextAppender) {
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
