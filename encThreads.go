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

type WorkerEncodeThreadData struct {
	e    *DiffRLEEncoder
	data []byte
	diff bool
}

func WorkerEncodeThread(c chan WorkerEncodeThreadData) {
	for {
		d, ok := <-c
		if !ok {
			return
		}
		header := HEADER_NORMAL
		if d.diff {
			header |= HEADER_DIFF
		}
		d.e.BeginFrame(header, 2, d.data[0])
		for _, x := range d.data[1:] {
			d.e.WriteCrumb(x)
		}
		c <- d
	}
}
