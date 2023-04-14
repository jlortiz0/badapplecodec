package vidcomp

type DiffRLEEncoder struct {
	*CrumbRLEEncoder
	curBit bool
}

func (e *DiffRLEEncoder) BeginFrame(header uint32, headerLen int, crumb byte) {
	e.curBit = false
	e.CrumbRLEEncoder.BeginFrame(header, headerLen, crumb)
}

func (e *DiffRLEEncoder) WriteCrumb(b byte) error {
	if b&0xfc != 0 {
		return ErrNotCrumb
	}
	b1 := b&2 != 0
	b2 := b&1 != 0
	b = 0
	if b1 != e.curBit {
		b |= 2
		e.curBit = b1
	}
	if b2 != e.curBit {
		b |= 1
		e.curBit = b2
	}
	return e.WriteCrumb(b)
}

type DiffRLEDecoder struct {
	*CrumbRLEDecoder
	curBit bool
}

func (d *DiffRLEDecoder) ReadHeader(bits int) (uint32, bool) {
	d.curBit = false
	return d.CrumbRLEDecoder.ReadHeader(bits)
}

func (d *DiffRLEDecoder) ReadCrumb() (byte, bool) {
	b, e := d.CrumbRLEDecoder.ReadCrumb()
	if !e {
		return b, e
	}
	b1 := b&2 != 0
	b2 := b&1 != 0
	b = 0
	if b1 {
		d.curBit = !d.curBit
	}
	if d.curBit {
		b |= 2
	}
	if b2 {
		d.curBit = !d.curBit
	}
	if d.curBit {
		b |= 1
	}
	return b, false
}
