package main

import "io"

type DiffRLEEncoder struct {
	*CrumbRLEEncoder
	lastCrumb byte
	width     int
	fBit      bool
	pos       int
}

func NewDiffRLEEncoder(width int) *DiffRLEEncoder {
	return &DiffRLEEncoder{NewCrumbRLEEncoder(), 0, width, false, 0}
}

func (e *DiffRLEEncoder) BeginFrame(header uint32, headerLen int, b byte) {
	e.lastCrumb = b
	e.pos = 1
	e.fBit = header&1 != 0
	e.CrumbRLEEncoder.BeginFrame(header, headerLen, b)
}

func (e *DiffRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	if e.pos == e.width {
		e.pos = 0
		if e.fBit {
			e.lastCrumb = 3
		} else {
			e.lastCrumb = 0
		}
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
	width     int
	fBit      bool
	pos       int
}

func NewDiffRLEDecoder(data io.ByteReader, width int) *DiffRLEDecoder {
	return &DiffRLEDecoder{NewCrumbRLEDecoder(data), 0, width, false, 0}
}

func (d *DiffRLEDecoder) ReadHeader(bits int) (uint32, bool) {
	d.pos = d.width
	header, ok := d.CrumbRLEDecoder.ReadHeader(bits)
	if ok {
		d.fBit = header&1 != 0
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
		if d.fBit {
			d.lastCrumb = 3
		} else {
			d.lastCrumb = 0
		}
	}
	d.pos++
	b ^= d.lastCrumb
	d.lastCrumb = b
	return b, true
}

// func (d DiffRLEDecoder) Copy() DiffRLEDecoder {
// 	return DiffRLEDecoder{CrumbRLEDecoder: d.CrumbRLEDecoder.Copy(), curBit: d.curBit}
// }
