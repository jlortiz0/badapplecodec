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

var ccColors = []byte{0x19, 0x4C, 0x99, 0xF0}

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
			bestInd := 0
			bestDiff := byte(255)
			for j, x := range ccColors {
				diff := int16(b2[i*4]) - int16(x)
				if diff < 0 {
					diff = -diff
				}
				if byte(diff) < bestDiff {
					bestDiff = byte(diff)
					bestInd = j
				}
			}
			b[i] = byte(bestInd)
		}
		split1, split2 := splitNibbles(b)
		c1 <- WorkerEncodeThreadData{e: e.Copy(), data1: split1, data2: split2}
		split1, split2 = splitNibbles(XORTheseArrays(b, lastFrame))
		c2 <- WorkerEncodeThreadData{e: e.Copy(), data1: split1, data2: split2, diff: true}
		d1 := <-c1
		d2 := <-c2
		if d1.e.Len() < d2.e.Len() {
			e = d1.e
		} else {
			e = d2.e
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
	defer close(c)
	buf := bufio.NewReader(rd)
	lastFrame := make([]byte, 4)
	buf.Read(lastFrame)
	if lastFrame[0] != 'J' || lastFrame[1] != 'B' || lastFrame[2] != 'A' || lastFrame[3] != 'C' {
		return
	}
	buf.Read(lastFrame)
	c <- lastFrame
	h := int(binary.BigEndian.Uint16(lastFrame))
	w := int(binary.BigEndian.Uint16(lastFrame[2:]))
	lastFrame = make([]byte, h*w)
	d := NewDiffRLEDecoder(buf)
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
		for i, x := range b {
			temp[i] = ccColors[x]
		}
		c <- temp
	}
}

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "play", "badapple.bac")
		main2()
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
	if err.Error() != "End of file" {
		panic(err)
	}
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
	if len(temp) == 0 {
		fmt.Println("Invalid file!")
		return
	}
	h := int32(binary.BigEndian.Uint16(temp))
	w := int32(binary.BigEndian.Uint16(temp[2:]))

	err = sdl.Init(sdl.INIT_TIMER | sdl.INIT_VIDEO)
	if err != nil {
		panic(err)
	}
	defer sdl.Quit()
	sdl.EventState(sdl.MOUSEMOTION, sdl.DISABLE)
	sdl.EventState(sdl.KEYUP, sdl.DISABLE)
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "0")

	window, display, err := sdl.CreateWindowAndRenderer(1024, 768, sdl.WINDOW_SHOWN)
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
	sx := w
	sy := h
	if h*1024 >= w*768 {
		sy = 768
		sx = 768 * w / h
	} else {
		sx = 1024
		sy = 1024 * h / w
	}
	temp2 := make([]byte, h*w*4)
	t := time.Tick(time.Second / 30)
	display.SetDrawColor(0, 0, 0, 0)
	waitMode := false
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
		tex.Update(&sdl.Rect{H: h, W: w}, unsafe.Pointer(&temp2[0]), int(w)*4)
		display.Clear()
		display.Copy(tex, nil, &sdl.Rect{X: (1024 - sx) / 2, Y: (768 - sy) / 2, H: sy, W: sx})
		display.Present()
		event := sdl.PollEvent()
		if waitMode && event == nil {
			event = &sdl.UserEvent{}
		}
		for event != nil {
			if event.GetType() == sdl.QUIT {
				return
			}
			if event.GetType() == sdl.KEYDOWN {
				ev := event.(*sdl.KeyboardEvent)
				switch ev.Keysym.Sym {
				case sdl.K_ESCAPE:
					return
				case sdl.K_b:
					XORTheseArrays(nil, nil)
				case sdl.K_SPACE:
					waitMode = !waitMode
				case sdl.K_RETURN:
					fallthrough
				case sdl.K_RETURN2:
					event = nil
					continue
				}
			}
			event = sdl.PollEvent()
			if event == nil && waitMode {
				event = sdl.WaitEvent()
			}
		}
		<-t
	}
	event := sdl.WaitEvent()
	for event.GetType() != sdl.KEYDOWN && event.GetType() != sdl.QUIT {
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
