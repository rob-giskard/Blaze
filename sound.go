package main

import (
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 44100

type Sounds struct {
	ctx                            *audio.Context
	jump, collect, hit, out, click []byte
	shield, block, win, pop        []byte
}

// synth makes a short 16-bit stereo clip. kind: 0 sine, 1 square, 2 noise.
func synth(f0, f1, dur, vol float64, kind int) []byte {
	n := int(dur * sampleRate)
	b := make([]byte, n*4)
	phase := 0.0
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n)
		f := f0 + (f1-f0)*t
		phase += 2 * math.Pi * f / sampleRate
		var v float64
		switch kind {
		case 0:
			v = math.Sin(phase)
		case 1:
			if math.Sin(phase) >= 0 {
				v = 0.6
			} else {
				v = -0.6
			}
		case 2:
			v = (rand.Float64()*2 - 1) * (1 - t) * 0.8
			v += math.Sin(phase) * 0.6
		}
		env := math.Min(1, float64(i)/(0.005*sampleRate)) * math.Pow(1-t, 1.6)
		s := int16(v * env * vol * 32767)
		b[i*4], b[i*4+1] = byte(s), byte(s>>8)
		b[i*4+2], b[i*4+3] = byte(s), byte(s>>8)
	}
	return b
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func newSounds() *Sounds {
	return &Sounds{
		ctx:     audio.NewContext(sampleRate),
		jump:    synth(320, 620, 0.12, 0.25, 0),
		collect: concat(synth(880, 880, 0.07, 0.22, 0), synth(1320, 1320, 0.14, 0.22, 0)),
		hit:     synth(160, 60, 0.28, 0.45, 2),
		out:     concat(synth(440, 330, 0.18, 0.25, 1), synth(330, 220, 0.22, 0.25, 1), synth(220, 110, 0.4, 0.25, 1)),
		click:   synth(700, 900, 0.05, 0.18, 0),
		shield:  concat(synth(523, 523, 0.07, 0.22, 0), synth(659, 659, 0.07, 0.22, 0), synth(784, 784, 0.07, 0.22, 0), synth(1047, 1047, 0.2, 0.22, 0)),
		block:   concat(synth(1400, 700, 0.08, 0.25, 1), synth(900, 900, 0.18, 0.18, 0)),
		win: concat(synth(523, 523, 0.12, 0.24, 1), synth(659, 659, 0.12, 0.24, 1), synth(784, 784, 0.12, 0.24, 1),
			synth(1047, 1047, 0.18, 0.24, 1), synth(784, 784, 0.1, 0.24, 1), synth(1047, 1047, 0.5, 0.24, 1)),
		pop: synth(300, 80, 0.15, 0.2, 2),
	}
}

func (s *Sounds) play(b []byte) {
	if s == nil || s.ctx == nil {
		return
	}
	p := s.ctx.NewPlayerFromBytes(b)
	p.Play()
}
