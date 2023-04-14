package badapplecodec

import (
	"bufio"
	"os"

	"github.com/jlortiz0/multisav/streamy"
)

func main() {
	s, err := streamy.NewAvVideoReader("apple.mp4", "", false)
	if err != nil {
		panic(err)
	}
	defer s.Destroy()
	h, w := s.GetDimensions()
	b := make([]byte, h*w)
	b2 := make([]byte, h*w*4)

	f, err := os.Create("out.bac")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	defer buf.Flush()

	c1 := make(chan WorkerEncodeThreadData)
	c2 := make(chan WorkerEncodeThreadData)
	go WorkerEncodeThread(c1)
	go WorkerEncodeThread(c2)
	defer close(c1)
	defer close(c2)

	e := NewDiffRLEEncoder()
	lastFrame := make([]byte, len(b))
	for {
		err = s.Read(b2)
		if err != nil {
			break
		}
		for i := 0; i < len(b); i++ {
			b[i] = b2[i*4] >> 6
		}
		split1, split2 := splitNibbles(b)
		c1 <- WorkerEncodeThreadData{e: e.Copy(), data1: split1, data2: split2}
		split1, split2 = splitNibbles(XORTheseArrays(b, lastFrame))
		c2 <- WorkerEncodeThreadData{e: e, data1: split1, data2: split2, diff: true}
		d1 := <-c1
		d2 := <-c2
		if d1.e.Len() < d2.e.Len() {
			e = d1.e
		}
		e.Flush(buf)
		temp := b
		b = lastFrame
		lastFrame = temp
	}
	e.Finalize()
	e.Flush(buf)
}

func splitNibbles(b []byte) ([]byte, []byte) {
	out1 := make([]byte, len(b)/2)
	out2 := make([]byte, len(b)/2)
	for i := 0; i < len(b)/2; i++ {
		var b1, b2 byte
		temp := b[2*i]
		if temp&2 != 0 {
			b1 |= 2
		}
		if temp&1 != 0 {
			b2 |= 2
		}
		temp = b[2*i+1]
		if temp&2 != 0 {
			b1 |= 1
		}
		if temp&1 != 0 {
			b2 |= 1
		}
		out1[i] = b1
		out2[i] = b2
	}
	return out1, out2
}
