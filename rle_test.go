package vidcomp_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/jlortiz0/vidcomp"
)

func TestEncoder(t *testing.T) {
	data := []byte{0, 1, 0, 0, 0, 0}
	e := vidcomp.NewCrumbRLEEncoder()
	e.BeginFrame(3, 2, data[0])
	for _, x := range data[1:] {
		e.WriteCrumb(x)
	}
	e.Finalize()
	b := new(bytes.Buffer)
	e.Flush(b)
	out := binary.BigEndian.Uint16(b.Bytes())
	if out != 0x4312 {
		t.Error("data mismatch", 0x4312, out)
	}
}

func TestDecoder(t *testing.T) {
	data := []byte{0x43, 0x12}
	d := vidcomp.NewCrumbRLEDecoder(bytes.NewReader(data))
	header, b := d.ReadHeader(2)
	if !b {
		t.Fatal("read error")
	}
	if header != 3 {
		t.Error("header mismatch", 3, header)
	}
	data = make([]byte, 6)
	for i := 0; i < len(data); i++ {
		b2, e := d.ReadCrumb()
		if !e {
			t.Fatal("read error")
		}
		data[i] = b2
	}
	if len(data) != 6 {
		t.Error("data len mismatch", 6, len(data))
	}
	if !bytes.Equal(data, []byte{0, 1, 0, 0, 0, 0}) {
		t.Error("data mismatch", []byte{0, 1, 0, 0, 0, 0}, data)
	}
}
