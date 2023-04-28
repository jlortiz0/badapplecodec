package main

import "io"

type DiffRLEEncoder struct {
	*CrumbRLEEncoder
	lastCrumb byte
}

func NewDiffRLEEncoder() *DiffRLEEncoder {
	return &DiffRLEEncoder{NewCrumbRLEEncoder(), 0}
}

func (e *DiffRLEEncoder) BeginFrame(header uint32, headerLen int, b byte) {
	e.lastCrumb = b
	e.CrumbRLEEncoder.BeginFrame(header, headerLen, b)
}

func (e *DiffRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	b2 := b
	b ^= e.lastCrumb
	e.lastCrumb = b2
	return e.CrumbRLEEncoder.WriteCrumb(b)
}

// func (d *DiffRLEEncoder) Copy() *DiffRLEEncoder {
// 	return &DiffRLEEncoder{CrumbRLEEncoder: d.CrumbRLEEncoder.Copy(), lastCrumb: d.lastCrumb}
// }

type DiffRLEDecoder struct {
	*CrumbRLEDecoder
	lastCrumb byte
}

func NewDiffRLEDecoder(data io.ByteReader) *DiffRLEDecoder {
	return &DiffRLEDecoder{NewCrumbRLEDecoder(data), 0}
}

func (d *DiffRLEDecoder) ReadHeader(bits int) (uint32, bool) {
	d.lastCrumb = 0
	return d.CrumbRLEDecoder.ReadHeader(bits)
}

func (d *DiffRLEDecoder) ReadCrumb() (byte, bool) {
	b, e := d.CrumbRLEDecoder.ReadCrumb()
	if !e {
		return b, e
	}
	b ^= d.lastCrumb
	d.lastCrumb = b
	return b, true
}

// func (d DiffRLEDecoder) Copy() DiffRLEDecoder {
// 	return DiffRLEDecoder{CrumbRLEDecoder: d.CrumbRLEDecoder.Copy(), curBit: d.curBit}
// }
