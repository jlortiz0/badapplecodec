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
	b := make([]byte, (h*w)/2)
	lastFrame := make([]byte, len(b))

	for {
		b2, ok := <-c
		if !ok {
			break
		}
		for i := 0; i < len(b2); i += 8 {
			var a byte
			if b2[i] & 0x80 != 0 {
				a += 2
			}
			if b2[i + 4] & 0x80 != 0 {
				a += 1
			}
			b[i / 8] = a
		}
		c1 <- WorkerEncodeThreadData{e: e.Copy(), data: b}
		diffed := XORTheseArrays(b, lastFrame)
		c2 <- WorkerEncodeThreadData{e: e.Copy(), data: diffed, diff: true}
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
	lastFrame = make([]byte, (h*w)/2)
	d := NewDiffRLEDecoder(buf)
	b := make([]byte, (h*w)/2)

	for {
		header, e := d.ReadHeader(2)
		if !e {
			break
		}
		for i := 0; i < len(b); i++ {
			b[i], e = d.ReadCrumb()
			if !e {
				return
			}
		}
		if header&HEADER_DIFF != 0 {
			b = XORTheseArrays(b, lastFrame)
		}

		temp := make([]byte, len(b) * 2)
		for i, x := range b {
			if x & 2 != 0 {
				temp[2 * i] = 0xFF
			}
			if x & 1 != 0 {
				temp[2 * i + 1] = 0xFF
			}
		}
		c <- temp
		temp = lastFrame
		lastFrame = b
		b = temp
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
	if os.Args[1] != "encode" || len(os.Args) < 4 {
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
	h2 := h
	h = (h * 4) / 3
	sx := w
	sy := h
	if h*1024 >= w*768 {
		sy = 768
		sx = 768 * w / h
	} else {
		sx = 1024
		sy = 1024 * h / w
	}
	h = h2
	temp2 := make([]byte, h*w*4)
	var t <-chan time.Time
	if h > 20 {
		t = time.Tick(time.Second / 10)
	} else {
		t = time.Tick(time.Second / 20)
	}
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

