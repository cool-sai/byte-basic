package runtime

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	MsgCall      byte = 1
	MsgReply     byte = 2
	MsgException byte = 3
	maxFrame          = 1 << 20
)

type message struct {
	typ    byte
	seq    uint32
	method string
	body   []byte
}

func writeMsg(w io.Writer, typ byte, seq uint32, method string, body []byte) error {
	mb := []byte(method)
	n := 1 + 4 + 2 + len(mb) + len(body)
	buf := make([]byte, 4+n)
	binary.BigEndian.PutUint32(buf[0:4], uint32(n))
	buf[4] = typ
	binary.BigEndian.PutUint32(buf[5:9], seq)
	binary.BigEndian.PutUint16(buf[9:11], uint16(len(mb)))
	copy(buf[11:11+len(mb)], mb)
	copy(buf[11+len(mb):], body)
	_, err := w.Write(buf)
	return err
}

func readMsg(r io.Reader) (message, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return message{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n < 7 || n > maxFrame {
		return message{}, fmt.Errorf("runtime: bad frame %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return message{}, err
	}
	typ := buf[0]
	seq := binary.BigEndian.Uint32(buf[1:5])
	mlen := int(binary.BigEndian.Uint16(buf[5:7]))
	if 7+mlen > len(buf) {
		return message{}, fmt.Errorf("runtime: short method")
	}
	return message{
		typ:    typ,
		seq:    seq,
		method: string(buf[7 : 7+mlen]),
		body:   buf[7+mlen:],
	}, nil
}
