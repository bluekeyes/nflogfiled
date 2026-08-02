package message

import "encoding"

func formatMessage(w writer, msg Message) {
	// TODO(bkeyes): only write fields that are present and selected
	w.Open()
	w.UintField("group", uint64(msg.Group))
	w.TextField("timestamp", msg.ReceivedAt)
	w.TextField("src_ip", msg.Packet.SrcIP)
	w.UintField("src_port", uint64(msg.Packet.SrcPort))
	w.TextField("dst_ip", msg.Packet.DstIP)
	w.UintField("src_port", uint64(msg.Packet.DstPort))
	w.Close()
}

type writer interface {
	Open()
	StringField(string, string)
	UintField(string, uint64)
	TextField(string, encoding.TextAppender)
	RawField(string, []byte)
	Close()
}
