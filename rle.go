package main

import (
	"C"
	"bytes"
	"errors"
)

var ErrNotCrumb error = errors.New("not a crumb")

type CrumbRLEEncoder struct {
	output    *bytes.Buffer
	packetLen int
	bytePos   byte
	curByte   byte
}

const (
	encoder_flag_init byte = 1 << 7 // Set if at least one crumb has been written
	encoder_flag_mask byte = encoder_flag_init - 1
)

func NewCrumbRLEEncoder(header uint32, headerLen int) *CrumbRLEEncoder {
	out := &CrumbRLEEncoder{output: new(bytes.Buffer)}
	for i := 0; i < headerLen; i++ {
		out.writeBit(header&(1<<i) != 0)
	}
	return out
}

func (e *CrumbRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	if e.bytePos&encoder_flag_init == 0 {
		e.bytePos = encoder_flag_init
		e.writeBit(b != 0)
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
		e.curByte |= 1 << (e.bytePos & encoder_flag_mask)
	}
	if e.bytePos&encoder_flag_mask == 7 {
		e.output.WriteByte(e.curByte)
		e.curByte = 0
		e.bytePos &= ^encoder_flag_mask
	} else {
		e.bytePos++
	}
}

func (e *CrumbRLEEncoder) Flush() {
	e.flushPacket()
	for e.bytePos&encoder_flag_mask != 0 {
		e.writeBit(false)
	}
}

func (e *CrumbRLEEncoder) flushPacket() {
	if e.packetLen != 0 {
		e.packetLen++
		pos := 63 - int(C.__builtin_clzll(C.ulonglong(e.packetLen)))
		for i := 2; i < pos; i++ {
			e.writeBit(true)
		}
		e.writeBit(false)
		for i := pos - 2; i >= 0; i-- {
			e.writeBit(e.packetLen&(1<<i) != 0)
		}
		e.packetLen = 0
	}
}

func (e *CrumbRLEEncoder) Len() int {
	return e.output.Len()
}

func (e *CrumbRLEEncoder) Bytes() []byte {
	e.Flush()
	return e.output.Bytes()
}

type CrumbRLEDecoder struct {
	data      *bytes.Reader
	packetLen int
	curByte   byte
	bytePos   byte
}

func NewCrumbRLEDecoder(data *bytes.Reader) *CrumbRLEDecoder {
	out := &CrumbRLEDecoder{data: data}
	out.bytePos = 8
	if b, _ := out.readBit(); !b {
		out.beginRLEPacket()
	}
	return out
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
	for !b {
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
