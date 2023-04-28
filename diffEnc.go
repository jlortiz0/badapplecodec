package main

import "io"

type DiffRLEEncoder struct {
	*CrumbRLEEncoder
	lastCrumb byte
	width     int
	mode      byte
	pos       int
}

func NewDiffRLEEncoder(width int) *DiffRLEEncoder {
	return &DiffRLEEncoder{CrumbRLEEncoder: NewCrumbRLEEncoder(), width: width}
}

func (e *DiffRLEEncoder) BeginFrame(header uint32, headerLen int, b byte, mode byte) {
	e.lastCrumb = b
	e.pos = 1
	e.mode = mode
	e.CrumbRLEEncoder.BeginFrame(header<<2+uint32(mode), headerLen+2, b^e.mode)
}

func (e *DiffRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	if e.pos == e.width {
		e.pos = 0
		e.lastCrumb = e.mode
	}
	e.pos++
	b2 := b
	b ^= e.lastCrumb
	e.lastCrumb = b2
	return e.CrumbRLEEncoder.WriteCrumb(b)
}

// func (d *DiffRLEEncoder) Copy() *DiffRLEEncoder {
// 	return &DiffRLEEncoder{CrumbRLEEncoder: d.CrumbRLEEncoder.Copy(), lastCrumb: d.lastCrumb, width: d.width, fBit: d.fBit, pos: d.pos}
// }

type DiffRLEDecoder struct {
	*CrumbRLEDecoder
	lastCrumb byte
	mode      byte
	width     int
	pos       int
}

func NewDiffRLEDecoder(data io.ByteReader, width int) *DiffRLEDecoder {
	return &DiffRLEDecoder{CrumbRLEDecoder: NewCrumbRLEDecoder(data), width: width}
}

func (d *DiffRLEDecoder) ReadHeader(bits int) (uint32, bool) {
	d.pos = d.width
	header, ok := d.CrumbRLEDecoder.ReadHeader(bits + 2)
	if ok {
		d.mode = byte(header & 3)
		header >>= 2
	}
	return header, ok
}

func (d *DiffRLEDecoder) ReadCrumb() (byte, bool) {
	b, e := d.CrumbRLEDecoder.ReadCrumb()
	if !e {
		return b, e
	}
	if d.pos == d.width {
		d.pos = 0
		d.lastCrumb = d.mode
	}
	d.pos++
	b ^= d.lastCrumb
	d.lastCrumb = b
	return b, true
}

// func (d DiffRLEDecoder) Copy() DiffRLEDecoder {
// 	return DiffRLEDecoder{CrumbRLEDecoder: d.CrumbRLEDecoder.Copy(), curBit: d.curBit}
// }
