package reels

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
)

// Our own epic score, synthesised from scratch so a reel never carries third-party audio that
// Facebook's rights matching could mute: an opening impact, a driving low-string ostinato,
// taiko hits, a minor i–VI–III–VII string pad and brass stabs, with a short reverb tail.

const sampleRate = 44100

type stem struct{ left, right []float64 }

func newStem(seconds float64) *stem {
	n := int(seconds * sampleRate)
	return &stem{make([]float64, n), make([]float64, n)}
}

func (t *stem) add(start int, left, right []float64) {
	for i := range left {
		if j := start + i; j >= 0 && j < len(t.left) {
			t.left[j] += left[i]
			t.right[j] += right[i]
		}
	}
}

func midiHz(note float64) float64 { return 440 * math.Pow(2, (note-69)/12) }

// saw is a band-limited-enough sawtooth: the voices are low-passed afterwards.
func saw(phase float64) float64 { return 2 * (phase - math.Floor(phase+0.5)) }

type onePole struct{ y float64 }

func (f *onePole) run(x, cutoff float64) float64 {
	a := 1 - math.Exp(-2*math.Pi*cutoff/sampleRate)
	f.y += a * (x - f.y)
	return f.y
}

// voice renders detuned saws for one note, low-passed and shaped by an attack/release envelope.
func voice(note, seconds, attack, release, cutoff, gain, pan float64, detune []float64) ([]float64, []float64) {
	n := int(seconds * sampleRate)
	left, right := make([]float64, n), make([]float64, n)
	var lp1, lp2 onePole
	phases := make([]float64, len(detune))
	for i := range phases {
		phases[i] = rand.Float64()
	}
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		var s float64
		for k, cents := range detune {
			phases[k] += midiHz(note) * math.Pow(2, cents/1200) / sampleRate
			s += saw(phases[k])
		}
		s /= float64(len(detune))
		s = lp2.run(lp1.run(s, cutoff), cutoff)
		env := math.Min(1, t/attack)
		if rest := seconds - t; rest < release {
			env *= rest / release
		}
		s *= env * gain
		left[i], right[i] = s*(1-pan)*0.5*2, s*(1+pan)*0.5*2
	}
	return left, right
}

// drum renders a pitched-down sine body with a noise transient: taiko when deep, impact when huge.
func drum(startHz, endHz, seconds, decay, noise, gain float64) ([]float64, []float64) {
	n := int(seconds * sampleRate)
	out := make([]float64, n)
	var phase float64
	var lp onePole
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		hz := endHz + (startHz-endHz)*math.Exp(-t*18)
		phase += hz / sampleRate
		body := math.Sin(2*math.Pi*phase) * math.Exp(-t/decay)
		hit := lp.run(rand.Float64()*2-1, 2500) * math.Exp(-t*40) * noise
		out[i] = math.Tanh((body+hit)*1.6) * gain
	}
	return out, append([]float64(nil), out...)
}

// reverb is a small Schroeder reverb: four parallel combs into two allpasses, per channel.
func reverb(in []float64, mix float64, offset int) []float64 {
	combs := []int{1557, 1617, 1491, 1422}
	out := make([]float64, len(in))
	for _, length := range combs {
		length += offset
		buf := make([]float64, length)
		var store float64
		for i, x := range in {
			y := buf[i%length]
			store = y*0.8 + store*0.2
			buf[i%length] = x + store*0.84
			out[i] += y / 4
		}
	}
	for _, length := range []int{225 + offset/4, 556 + offset/4} {
		buf := make([]float64, length)
		for i, x := range out {
			y := buf[i%length]
			buf[i%length] = x + y*0.5
			out[i] = y - x*0.5
		}
	}
	for i := range out {
		out[i] = in[i] + out[i]*mix
	}
	return out
}

// EpicScore renders a cinematic bed of the given length as 16-bit stereo WAV bytes.
func EpicScore(seconds float64, seed int64) []byte {
	rand.Seed(seed)
	const bpm = 96.0
	beat := 60 / bpm
	bar := beat * 4
	// D minor: Dm – Bb – F – C (i–VI–III–VII); MIDI roots in octave 2.
	chords := [][]float64{{38, 50, 53, 57}, {34, 50, 53, 58}, {41, 48, 53, 57}, {36, 48, 52, 55}}
	pad, rhythm := newStem(seconds+3), newStem(seconds+3)
	at := func(t float64) int { return int(t * sampleRate) }

	// Opening impact and a deep boom so the first frame lands hard.
	l, r := drum(140, 32, 3.5, 0.9, 1.0, 0.7)
	rhythm.add(0, l, r)

	for b := 0; float64(b)*bar < seconds; b++ {
		start := float64(b) * bar
		chord := chords[b%len(chords)]
		// String pad: each chord tone on its own detuned voice, spread across the stereo field.
		for k, note := range chord {
			pan := []float64{0, -0.5, 0.5, -0.2}[k]
			l, r := voice(note+12, bar+0.6, 0.35, 0.6, 1800, 0.2, pan, []float64{-9, 0, 7})
			pad.add(at(start), l, r)
		}
		// Low string ostinato in eighth notes on the root.
		for e := 0; e < 8; e++ {
			accent := 0.24
			if e%2 == 0 {
				accent = 0.32
			}
			l, r := voice(chord[0]+12+12*float64(e%2), beat/2*0.9, 0.005, 0.08, 1300, accent, 0, []float64{-5, 5})
			rhythm.add(at(start+float64(e)*beat/2), l, r)
		}
		// Taiko on beats 1 and 3, a sixteenth-note fill at the end of every second bar.
		for _, bt := range []float64{0, 2} {
			l, r := drum(95, 48, 0.9, 0.28, 0.4, 0.42)
			rhythm.add(at(start+bt*beat), l, r)
		}
		if b%2 == 1 {
			for s := 0; s < 4; s++ {
				l, r := drum(110, 55, 0.5, 0.15, 0.5, 0.2+0.06*float64(s))
				rhythm.add(at(start+3*beat+float64(s)*beat/4), l, r)
			}
		}
		// Brass stab on each downbeat after the first bar: root and fifth, bright then closing.
		if b > 0 {
			for _, note := range []float64{chord[0] + 12, chord[0] + 19, chord[0] + 24} {
				l, r := voice(note, beat*1.5, 0.04, 0.9, 2600, 0.17, 0, []float64{-12, -3, 4, 11})
				pad.add(at(start), l, r)
			}
		}
	}

	left := reverb(pad.left, 0.45, 0)
	right := reverb(pad.right, 0.45, 23)
	rl := reverb(rhythm.left, 0.15, 0)
	rr := reverb(rhythm.right, 0.15, 23)
	n := int(seconds * sampleRate)
	mixL, mixR := make([]float64, n), make([]float64, n)
	var peak float64
	for i := 0; i < n; i++ {
		mixL[i] = math.Tanh((left[i] + rl[i]) * 1.2)
		mixR[i] = math.Tanh((right[i] + rr[i]) * 1.2)
		// Short fade in, 1.2 s fade out so the reel loops cleanly.
		t := float64(i) / sampleRate
		fade := math.Min(1, t/0.01) * math.Min(1, (seconds-t)/1.2)
		mixL[i] *= fade
		mixR[i] *= fade
		peak = math.Max(peak, math.Max(math.Abs(mixL[i]), math.Abs(mixR[i])))
	}
	gain := 0.89 / math.Max(peak, 1e-9)

	var buf bytes.Buffer
	dataSize := uint32(n * 4)
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, 36+dataSize)
	buf.WriteString("WAVEfmt ")
	for _, value := range []any{uint32(16), uint16(1), uint16(2), uint32(sampleRate), uint32(sampleRate * 4), uint16(4), uint16(16)} {
		binary.Write(&buf, binary.LittleEndian, value)
	}
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataSize)
	for i := 0; i < n; i++ {
		binary.Write(&buf, binary.LittleEndian, int16(mixL[i]*gain*32767))
		binary.Write(&buf, binary.LittleEndian, int16(mixR[i]*gain*32767))
	}
	return buf.Bytes()
}
