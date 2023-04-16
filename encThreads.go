package main

const (
	HEADER_NORMAL uint32 = 0
	HEADER_SWAP   uint32 = 1
	HEADER_DIFF   uint32 = 2
	HEADER_XOR    uint32 = 1
)

func XORTheseArrays(data, rodata []byte) []byte {
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ rodata[i]
	}
	return out
}

type WorkerWorkerEncodeThreadData struct {
	e    *DiffRLEEncoder
	data []byte
	diff bool
}

func WorkerWorkerEncodeThread(c chan WorkerWorkerEncodeThreadData) {
	for {
		d, ok := <-c
		if !ok {
			return
		}
		header := HEADER_SWAP
		if d.diff {
			header |= HEADER_DIFF
		}
		d.e.BeginFrame(header, 2, d.data[0])
		for _, x := range d.data[1:] {
			d.e.WriteCrumb(x)
		}
		c <- d
		d = <-c
		d.e.BeginFrame(HEADER_DIFF, 2, d.data[0])
		for _, x := range d.data[1:] {
			d.e.WriteCrumb(x)
		}
		c <- d
	}
}

type WorkerEncodeThreadData struct {
	e     *DiffRLEEncoder
	data1 []byte
	data2 []byte
	diff  bool
}

func WorkerEncodeThread(c chan WorkerEncodeThreadData) {
	c2 := make(chan WorkerWorkerEncodeThreadData)
	go WorkerWorkerEncodeThread(c2)
	defer close(c2)
	for {
		d, ok := <-c
		if !ok {
			return
		}
		c2 <- WorkerWorkerEncodeThreadData{e: d.e.Copy(), data: d.data2, diff: d.diff}
		header := HEADER_NORMAL
		if d.diff {
			header |= HEADER_DIFF
		}
		d.e.BeginFrame(header, 2, d.data1[0])
		for _, x := range d.data1[1:] {
			d.e.WriteCrumb(x)
		}
		d2 := <-c2
		swap := d.e.Len() > d2.e.Len()
		if swap {
			d.e = d2.e
			c2 <- WorkerWorkerEncodeThreadData{e: d.e.Copy(), data: d.data1}
		} else {
			c2 <- WorkerWorkerEncodeThreadData{e: d.e.Copy(), data: d.data2}
		}
		data := XORTheseArrays(d.data1, d.data2)
		d.e.BeginFrame(HEADER_XOR|HEADER_DIFF, 2, data[0])
		for _, x := range data[1:] {
			d.e.WriteCrumb(x)
		}
		d2 = <-c2
		if d.e.Len() > d2.e.Len() {
			d.e = d2.e
		}
		c <- d
	}
}
