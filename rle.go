package main

import (
	"C"
	"bytes"
	"errors"
)
import "io"

var ErrNotCrumb error = errors.New("not a crumb")

type CrumbRLEEncoder struct {
	output    *bytes.Buffer
	packetLen int
	bytePos   byte
	curByte   byte
}

func NewCrumbRLEEncoder() *CrumbRLEEncoder {
	out := &CrumbRLEEncoder{output: new(bytes.Buffer)}
	return out
}

func (e *CrumbRLEEncoder) BeginFrame(header uint32, headerLen int, crumb byte) {
	e.flushPacket()
	for i := 0; i < headerLen; i++ {
		e.writeBit(header&(1<<i) != 0)
	}
	if crumb == 0 {
		e.writeBit(false)
		e.packetLen++
	} else {
		e.writeBit(true)
		e.WriteCrumb(crumb)
	}
}

func (e *CrumbRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	if b == 0 {
		if e.packetLen == 0 {
			e.writeBit(false)
			e.writeBit(false)
		}
		e.packetLen++
		return nil
	}
	e.flushPacket()
	e.writeBit(b&2 != 0)
	e.writeBit(b&1 != 0)
	return nil
}

func (e *CrumbRLEEncoder) writeBit(b bool) {
	if b {
		e.curByte |= 1 << e.bytePos
	}
	if e.bytePos == 7 {
		e.output.WriteByte(e.curByte)
		e.curByte = 0
		e.bytePos = 0
	} else {
		e.bytePos++
	}
}

func (e *CrumbRLEEncoder) Flush(w io.Writer) {
	e.flushPacket()
	io.Copy(w, e.output)
	e.output.Reset()
}

func (e *CrumbRLEEncoder) Finalize() {
	e.flushPacket()
	for e.bytePos != 0 {
		e.writeBit(false)
	}
}

func (e *CrumbRLEEncoder) flushPacket() {
	if e.packetLen != 0 {
		e.packetLen++
		pos := 63 - int(C.__builtin_clzll(C.ulonglong(e.packetLen)))
		for i := 1; i < pos; i++ {
			e.writeBit(true)
		}
		e.writeBit(false)
		for i := pos - 1; i >= 0; i-- {
			e.writeBit(e.packetLen&(1<<i) != 0)
		}
		e.packetLen = 0
	}
}

func (e *CrumbRLEEncoder) Len() int {
	return e.output.Len() * 8 + int(e.bytePos)
}

func (e *CrumbRLEEncoder) Copy() *CrumbRLEEncoder {
	data := make([]byte, e.output.Len(), e.output.Cap())
	copy(data, e.output.Bytes())
	return &CrumbRLEEncoder{output: bytes.NewBuffer(data), packetLen: e.packetLen, bytePos: e.bytePos, curByte: e.curByte}
}

type CrumbRLEDecoder struct {
	data      io.ByteReader
	packetLen int
	curByte   byte
	bytePos   byte
}

func NewCrumbRLEDecoder(data io.ByteReader) *CrumbRLEDecoder {
	out := &CrumbRLEDecoder{data: data}
	out.bytePos = 8
	return out
}

func (d *CrumbRLEDecoder) ReadHeader(bits int) (uint32, bool) {
	var header uint32
	b, e := d.readBit()
	for i := 0; i < bits; i++ {
		if !e {
			return header, e
		}
		if b {
			header |= 1 << i
		}
		b, e = d.readBit()
	}
	if !b {
		d.beginRLEPacket()
	}
	return header, e
}

func (d *CrumbRLEDecoder) readBit() (bool, bool) {
	if d.bytePos == 8 {
		var err error
		d.curByte, err = d.data.ReadByte()
		if err != nil {
			return false, false
		}
		d.bytePos = 0
	}
	b := d.curByte&(1<<d.bytePos) != 0
	d.bytePos++
	return b, true
}

func (d *CrumbRLEDecoder) beginRLEPacket() {
	pos := 1
	b, e := d.readBit()
	for b {
		pos++
		b, e = d.readBit()
	}
	if !e {
		return
	}
	add := 0
	for i := 0; i < pos; i++ {
		add <<= 1
		b, _ = d.readBit()
		if b {
			add |= 1
		}
	}
	d.packetLen = (1 << pos) - 1 + add
}

func (d *CrumbRLEDecoder) ReadCrumb() (byte, bool) {
	if d.packetLen > 0 {
		d.packetLen--
		return 0, true
	}
	b1, e := d.readBit()
	if !e {
		return 0, false
	}
	b2, e := d.readBit()
	if !e {
		return 0, false
	}
	if !b1 && !b2 {
		d.beginRLEPacket()
		return d.ReadCrumb()
	}
	var b byte
	if b1 {
		b |= 2
	}
	if b2 {
		b |= 1
	}
	return b, true
}
