// Package wire is the on-the-wire encoding.
//
// Kitex's real codec is Thrift binary / compact / Protobuf. This is the same
// idea shrunk down: the network never sees Go structs or JSON keys, it sees
// field ids.
//
//	struct body = { type:u8, field_id:u16be, value }*  +  TStop
//	i64    value = 8-byte big-endian
//	string value = u32be length + bytes
//	bool   value = 0 or 1
package wire

import (
	"encoding/binary"
	"fmt"
)

const (
	TStop   byte = 0
	TBool   byte = 1
	TI64    byte = 2
	TString byte = 3
)

type Writer struct {
	buf []byte
}

func NewWriter() *Writer { return &Writer{} }

func (w *Writer) Bytes() []byte { return w.buf }

func (w *Writer) FieldI64(id int, v int64) {
	w.buf = append(w.buf, TI64)
	w.buf = binary.BigEndian.AppendUint16(w.buf, uint16(id))
	w.buf = binary.BigEndian.AppendUint64(w.buf, uint64(v))
}

func (w *Writer) FieldString(id int, v string) {
	w.buf = append(w.buf, TString)
	w.buf = binary.BigEndian.AppendUint16(w.buf, uint16(id))
	w.buf = binary.BigEndian.AppendUint32(w.buf, uint32(len(v)))
	w.buf = append(w.buf, v...)
}

func (w *Writer) FieldBool(id int, v bool) {
	w.buf = append(w.buf, TBool)
	w.buf = binary.BigEndian.AppendUint16(w.buf, uint16(id))
	if v {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

func (w *Writer) Stop() { w.buf = append(w.buf, TStop) }

type Reader struct {
	b []byte
	i int
}

func NewReader(b []byte) *Reader { return &Reader{b: b} }

func (r *Reader) NextField() (typ byte, id int, err error) {
	typ, err = r.u8()
	if err != nil {
		return 0, 0, err
	}
	if typ == TStop {
		return TStop, 0, nil
	}
	n, err := r.u16()
	if err != nil {
		return 0, 0, err
	}
	return typ, int(n), nil
}

func (r *Reader) ReadI64() (int64, error) {
	n, err := r.u64()
	return int64(n), err
}

func (r *Reader) ReadString() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	if r.i+int(n) > len(r.b) {
		return "", fmt.Errorf("wire: short string")
	}
	s := string(r.b[r.i : r.i+int(n)])
	r.i += int(n)
	return s, nil
}

func (r *Reader) ReadBool() (bool, error) {
	v, err := r.u8()
	return v == 1, err
}

func (r *Reader) Skip(typ byte) error {
	switch typ {
	case TI64:
		_, err := r.ReadI64()
		return err
	case TString:
		_, err := r.ReadString()
		return err
	case TBool:
		_, err := r.ReadBool()
		return err
	default:
		return fmt.Errorf("wire: skip unknown type %d", typ)
	}
}

func (r *Reader) u8() (byte, error) {
	if r.i >= len(r.b) {
		return 0, fmt.Errorf("wire: eof")
	}
	v := r.b[r.i]
	r.i++
	return v, nil
}

func (r *Reader) u16() (uint16, error) {
	if r.i+2 > len(r.b) {
		return 0, fmt.Errorf("wire: eof")
	}
	v := binary.BigEndian.Uint16(r.b[r.i:])
	r.i += 2
	return v, nil
}

func (r *Reader) u32() (uint32, error) {
	if r.i+4 > len(r.b) {
		return 0, fmt.Errorf("wire: eof")
	}
	v := binary.BigEndian.Uint32(r.b[r.i:])
	r.i += 4
	return v, nil
}

func (r *Reader) u64() (uint64, error) {
	if r.i+8 > len(r.b) {
		return 0, fmt.Errorf("wire: eof")
	}
	v := binary.BigEndian.Uint64(r.b[r.i:])
	r.i += 8
	return v, nil
}
