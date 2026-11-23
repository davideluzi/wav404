package sp404sx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Audio is decoded PCM, interleaved, normalized to [-1, 1).
type Audio struct {
	Rate     float64
	Channels int
	Samples  []float64
}

// Frames is the length in frames.
func (a *Audio) Frames() int { return len(a.Samples) / a.Channels }

// sampleFormat describes how one sample is stored.
type sampleFormat struct {
	bits     int // container size in bits
	float    bool
	unsigned bool // 8-bit WAV
	order    binary.ByteOrder
}

// pcmData is the raw sample data of a file plus its format, before decoding.
type pcmData struct {
	data     []byte
	channels int
	rate     float64
	sf       sampleFormat
}

// DecodeAudio decodes an uncompressed WAV (RIFF/WAVE) or AIFF/AIFC file.
func DecodeAudio(raw []byte) (*Audio, error) {
	p, err := parseAudio(raw)
	if err != nil {
		return nil, err
	}
	return decodePCM(p), nil
}

// parseAudio locates the sample data and format of a WAV or AIFF file.
func parseAudio(raw []byte) (*pcmData, error) {
	var p *pcmData
	var err error
	switch {
	case len(raw) < 12:
	case string(raw[0:4]) == "RIFF" && string(raw[8:12]) == "WAVE":
		p, err = parseWAV(raw)
	case string(raw[0:4]) == "FORM" && (string(raw[8:12]) == "AIFF" || string(raw[8:12]) == "AIFC"):
		p, err = parseAIFF(raw, string(raw[8:12]) == "AIFC")
	}
	if p == nil && err == nil {
		err = errors.New("not a WAV or AIFF file")
	}
	if err != nil {
		return nil, err
	}
	if p.channels < 1 {
		return nil, errors.New("no channels")
	}
	if !(p.rate > 0) || math.IsInf(p.rate, 0) {
		return nil, fmt.Errorf("invalid sample rate %v", p.rate)
	}
	size := (p.sf.bits + 7) / 8
	if p.sf.float && size != 4 && size != 8 || !p.sf.float && (size < 1 || size > 4) {
		return nil, fmt.Errorf("unsupported sample size %d bits", p.sf.bits)
	}
	return p, nil
}

// walkChunks calls fn for each chunk after the 12-byte container header.
// A chunk whose declared size runs past the end is truncated, as many
// writers leave a wrong size on the data chunk.
func walkChunks(raw []byte, order binary.ByteOrder, fn func(id string, body []byte)) {
	for p := 12; p+8 <= len(raw); {
		start := p + 8
		n := int(order.Uint32(raw[p+4:]))
		if n < 0 || n > len(raw)-start { // n < 0: overflow on 32-bit platforms
			n = len(raw) - start
		}
		fn(string(raw[p:p+4]), raw[start:start+n])
		p = start + n + n&1
	}
}

func parseWAV(raw []byte) (*pcmData, error) {
	le := binary.LittleEndian
	var fmtc, data []byte
	walkChunks(raw, le, func(id string, body []byte) {
		switch {
		case id == "fmt ":
			fmtc = body
		case id == "data" && data == nil:
			data = body
		}
	})
	if len(fmtc) < 16 {
		return nil, errors.New("WAV: missing fmt chunk")
	}
	if data == nil {
		return nil, errors.New("WAV: missing data chunk")
	}
	tag := le.Uint16(fmtc[0:])
	if tag == 0xFFFE { // WAVE_FORMAT_EXTENSIBLE
		if len(fmtc) < 26 {
			return nil, errors.New("WAV: truncated WAVE_FORMAT_EXTENSIBLE")
		}
		tag = le.Uint16(fmtc[24:]) // first two bytes of the subformat GUID
	}
	bits := int(le.Uint16(fmtc[14:]))
	sf := sampleFormat{bits: bits, order: le}
	switch tag {
	case 1:
		sf.unsigned = bits == 8
	case 3:
		sf.float = true
	default:
		return nil, fmt.Errorf("WAV: compressed or unsupported format (tag %#x)", tag)
	}
	return &pcmData{
		data:     data,
		channels: int(le.Uint16(fmtc[2:])),
		rate:     float64(le.Uint32(fmtc[4:])),
		sf:       sf,
	}, nil
}

func parseAIFF(raw []byte, aifc bool) (*pcmData, error) {
	be := binary.BigEndian
	var comm, ssnd []byte
	walkChunks(raw, be, func(id string, body []byte) {
		switch id {
		case "COMM":
			comm = body
		case "SSND":
			ssnd = body
		}
	})
	if len(comm) < 18 {
		return nil, errors.New("AIFF: missing COMM chunk")
	}
	if len(ssnd) < 8 {
		return nil, errors.New("AIFF: missing SSND chunk")
	}
	ch := int(be.Uint16(comm[0:]))
	frames := int(be.Uint32(comm[2:]))
	sf := sampleFormat{bits: int(be.Uint16(comm[6:])), order: be}
	if aifc && len(comm) >= 22 {
		switch c := string(comm[18:22]); c {
		case "NONE", "twos":
		case "sowt":
			sf.order = binary.LittleEndian
		case "fl32", "FL32":
			sf.float, sf.bits = true, 32
		case "fl64", "FL64":
			sf.float, sf.bits = true, 64
		default:
			return nil, fmt.Errorf("AIFC: compressed or unsupported format %q", c)
		}
	}
	offset := int(be.Uint32(ssnd[0:]))
	data := ssnd[8:]
	if offset > len(data) {
		return nil, errors.New("AIFF: bad SSND offset")
	}
	data = data[offset:]
	if want := frames * ch * ((sf.bits + 7) / 8); want >= 0 && want < len(data) { // want < 0: overflow
		data = data[:want]
	}
	return &pcmData{data: data, channels: ch, rate: extendedToFloat(comm[8:18]), sf: sf}, nil
}

// extendedToFloat converts an 80-bit IEEE 754 extended float (AIFF sample rate).
func extendedToFloat(b []byte) float64 {
	exp := int(binary.BigEndian.Uint16(b[0:]) & 0x7FFF)
	v := math.Ldexp(float64(binary.BigEndian.Uint64(b[2:])), exp-16383-63)
	if b[0]&0x80 != 0 {
		v = -v
	}
	return v
}

// decodePCM converts the samples to float64 in [-1, 1).
func decodePCM(p *pcmData) *Audio {
	sf := p.sf
	size := (sf.bits + 7) / 8
	n := len(p.data) / size
	n -= n % p.channels
	out := make([]float64, n)
	little := sf.order == binary.ByteOrder(binary.LittleEndian)
	for i := range out {
		b := p.data[i*size : (i+1)*size]
		switch {
		case sf.float && size == 4:
			out[i] = float64(math.Float32frombits(sf.order.Uint32(b)))
		case sf.float:
			out[i] = math.Float64frombits(sf.order.Uint64(b))
		default:
			// read big-endian into the top of a uint32, then reinterpret as
			// signed: the result is the sample scaled to 32 bit
			var u uint32
			for k := range size {
				if little {
					u = u<<8 | uint32(b[size-1-k])
				} else {
					u = u<<8 | uint32(b[k])
				}
			}
			u <<= 32 - 8*size
			if sf.unsigned {
				u ^= 1 << 31
			}
			out[i] = float64(int32(u)) / (1 << 31)
		}
	}
	return &Audio{Rate: p.rate, Channels: p.channels, Samples: out}
}
