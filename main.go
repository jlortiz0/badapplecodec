package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/jlortiz0/multisav/streamy"
	"github.com/veandco/go-sdl2/sdl"
)

func NewImageEncoder(c <-chan []byte, h, w int, wr io.Writer, closed chan<- struct{}) {
	buf := bufio.NewWriter(wr)
	defer buf.Flush()
	buf.Write(binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16([]byte{'J', 'B', 'A', 'C'}, uint16(h)), uint16(w)))

	c1 := make(chan WorkerEncodeThreadData)
	c2 := make(chan WorkerEncodeThreadData)
	go WorkerEncodeThread(c1)
	go WorkerEncodeThread(c2)
	defer close(c1)
	defer close(c2)

	e := NewDiffRLEEncoder()
	b := make([]byte, h*w)
	lastFrame := make([]byte, len(b))

	for {
		b2, ok := <-c
		if !ok {
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
	if closed != nil {
		close(closed)
	}
}

func NewImageDecoder(c chan<- []byte, rd io.Reader) {
	buf := bufio.NewReader(rd)
	lastFrame := make([]byte, 4)
	buf.Read(lastFrame)
	c <- lastFrame
	h := int(binary.BigEndian.Uint16(lastFrame))
	w := int(binary.BigEndian.Uint16(lastFrame[2:]))
	lastFrame = make([]byte, h*w)
	d := NewDiffRLEDecoder(buf)
	defer close(c)
	b1 := make([]byte, (h*w)/2)
	b2 := make([]byte, (h*w)/2)

	for {
		header1, e := d.ReadHeader(2)
		if !e {
			break
		}
		for i := 0; i < len(b1); i++ {
			b1[i], e = d.ReadCrumb()
			if !e {
				panic("unexpected EOF")
			}
		}
		header2, e := d.ReadHeader(2)
		if !e {
			panic("unexpected EOF")
		}
		for i := 0; i < len(b2); i++ {
			b2[i], e = d.ReadCrumb()
			if !e {
				panic("unexpected EOF")
			}
		}
		if header2&HEADER_XOR != 0 {
			b2 = XORTheseArrays(b2, b1)
		}
		if header1&HEADER_SWAP != 0 {
			temp := b1
			b1 = b2
			b2 = temp
		}
		b := combineNibbles(b1, b2)
		if header1&HEADER_DIFF != 0 {
			b = XORTheseArrays(b, lastFrame)
		}
		lastFrame = b

		temp := make([]byte, len(b))
		for i := 0; i < len(b); i++ {
			temp[i] = b[i] << 6
		}
		c <- temp
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:", os.Args[0], "encode <video> <output> or", os.Args[0], "play <.bac file>")
		return
	}
	if os.Args[1] == "play" {
		main2()
		return
	}
	if os.Args[1] != "encode" || len(os.Args) < 3 {
		fmt.Println("Usage:", os.Args[0], "encode <video> or", os.Args[0], "play <.bac file>")
		return
	}
	s, err := streamy.NewAvVideoReader(os.Args[2], "", false)
	if err != nil {
		panic(err)
	}
	defer s.Destroy()
	w, h := s.GetDimensions()
	b2 := make([]byte, h*w*4)
	if !strings.HasSuffix(os.Args[3], ".bac") {
		os.Args[3] += ".bac"
	}
	f, err := os.Create(os.Args[3])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	c := make(chan []byte)
	c2 := make(chan struct{})
	go NewImageEncoder(c, int(h), int(w), f, c2)
	err = s.Read(b2)
	for err == nil {
		c <- b2
		err = s.Read(b2)
	}
	fmt.Println(err)
	close(c)
	<-c2
}

func main2() {
	f, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	c := make(chan []byte)
	go NewImageDecoder(c, f)
	temp := <-c
	h := int32(binary.BigEndian.Uint16(temp))
	w := int32(binary.BigEndian.Uint16(temp[2:]))

	err = sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO)
	if err != nil {
		panic(err)
	}
	defer sdl.Quit()
	sdl.EventState(sdl.MOUSEMOTION, sdl.DISABLE)
	sdl.EventState(sdl.KEYUP, sdl.DISABLE)
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "best")

	window, display, err := sdl.CreateWindowAndRenderer(w, h, sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE)
	if err != nil {
		panic(err)
	}
	window.SetTitle("Bad Apple Encoder - " + os.Args[2])
	defer window.Destroy()
	defer display.Destroy()
	tex, err := display.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STREAMING, w, h)
	if err != nil {
		panic(err)
	}
	defer tex.Destroy()
	temp2 := make([]byte, h*w*4)
	t := time.Tick(time.Second / 30)
	for {
		temp, ok := <-c
		if !ok {
			break
		}
		for i, x := range temp {
			temp2[4*i] = x
			temp2[4*i+1] = x
			temp2[4*i+2] = x
		}
		tex.Update(&sdl.Rect{H: h, W: w}, unsafe.Pointer(&temp2[0]), int(w))
		display.Copy(tex, nil, &sdl.Rect{H: h, W: w})
		display.Present()
		<-t
	}
	event := sdl.WaitEvent()
	for event.GetType() != sdl.KEYDOWN {
		event = sdl.WaitEvent()
	}
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

func combineNibbles(b1, b2 []byte) []byte {
	out := make([]byte, len(b1)+len(b2))
	for i := 0; i < len(b1); i++ {
		var d1, d2 byte
		temp := b1[i]
		if temp&2 != 0 {
			d1 |= 2
		}
		if temp&1 != 0 {
			d2 |= 2
		}
		temp = b2[i]
		if temp&2 != 0 {
			d1 |= 1
		}
		if temp&1 != 0 {
			d2 |= 1
		}
		out[2*i] = d1
		out[2*i+1] = d2
	}
	return out
}
