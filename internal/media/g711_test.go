package media

import (
	"math"
	"testing"
)

func TestLinearToMuLawAndBack(t *testing.T) {
	// Roundtrip: encode then decode should be close to the original.
	// G.711 is lossy, so we check within quantization tolerance.
	samples := []int16{0, 1, -1, 100, -100, 1000, -1000, 8000, -8000, 32000, -32000}

	for _, orig := range samples {
		encoded := LinearToMuLaw(orig)
		decoded := MuLawToLinear(encoded)
		diff := math.Abs(float64(orig) - float64(decoded))
		// Allow generous tolerance for lossy codec
		tolerance := math.Abs(float64(orig)*0.15) + 40
		if diff > tolerance {
			t.Errorf("mu-law roundtrip: input=%d, encoded=0x%02x, decoded=%d, diff=%.0f > tolerance=%.0f",
				orig, encoded, decoded, diff, tolerance)
		}
	}
}

func TestLinearToMuLawKnownValues(t *testing.T) {
	// Silence in mu-law should be 0xFF (input 0)
	if got := LinearToMuLaw(0); got != 0xFF {
		t.Errorf("mu-law silence: got 0x%02x, want 0xFF", got)
	}

	// Positive and negative should differ only in sign bit (after complementing)
	posEnc := LinearToMuLaw(1000)
	negEnc := LinearToMuLaw(-1000)
	// The sign bit is bit 7 in the complemented value, so XOR should give 0x80
	if (posEnc ^ negEnc) != 0x80 {
		t.Errorf("mu-law sign symmetry: pos=0x%02x neg=0x%02x, XOR=0x%02x want 0x80",
			posEnc, negEnc, posEnc^negEnc)
	}
}

func TestLinearToMuLawClipping(t *testing.T) {
	// Values above MuLawClip should be clipped
	a := LinearToMuLaw(MuLawClip)
	b := LinearToMuLaw(MuLawClip + 100)
	if a != b {
		t.Errorf("mu-law clipping: MuLawClip encoded=0x%02x, MuLawClip+100 encoded=0x%02x, should be equal", a, b)
	}
}

func TestLinearToALawAndBack(t *testing.T) {
	samples := []int16{0, 1, -1, 100, -100, 1000, -1000, 8000, -8000, 32000, -32000}

	for _, orig := range samples {
		encoded := LinearToALaw(orig)
		decoded := ALawToLinear(encoded)
		diff := math.Abs(float64(orig) - float64(decoded))
		// A-law uses a different companding curve; higher values have larger quantization steps.
		// Tolerance is ~50% of input magnitude + baseline for near-zero values.
		tolerance := math.Abs(float64(orig)*0.55) + 40
		if diff > tolerance {
			t.Errorf("A-law roundtrip: input=%d, encoded=0x%02x, decoded=%d, diff=%.0f > tolerance=%.0f",
				orig, encoded, decoded, diff, tolerance)
		}
	}
}

func TestLinearToALawKnownValues(t *testing.T) {
	// Silence in A-law: encoding 0 then XOR with 0xD5
	got := LinearToALaw(0)
	// Verify it decodes back to near-zero
	decoded := ALawToLinear(got)
	if math.Abs(float64(decoded)) > 16 {
		t.Errorf("A-law silence: encoded=0x%02x decodes to %d, expected near 0", got, decoded)
	}
}

func TestLinearToALawSignSymmetry(t *testing.T) {
	posEnc := LinearToALaw(4000)
	negEnc := LinearToALaw(-4000)
	// After XOR with 0xD5, sign bit difference should be 0x80
	if (posEnc ^ negEnc) != 0x80 {
		t.Errorf("A-law sign symmetry: pos=0x%02x neg=0x%02x, XOR=0x%02x want 0x80",
			posEnc, negEnc, posEnc^negEnc)
	}
}

func TestEncodePCMToMuLaw(t *testing.T) {
	pcm := []int16{0, 100, -100, 5000, -5000}
	out := EncodePCMToMuLaw(pcm)
	if len(out) != len(pcm) {
		t.Fatalf("EncodePCMToMuLaw: got %d bytes, want %d", len(out), len(pcm))
	}
	for i, s := range pcm {
		want := LinearToMuLaw(s)
		if out[i] != want {
			t.Errorf("EncodePCMToMuLaw[%d]: got 0x%02x, want 0x%02x", i, out[i], want)
		}
	}
}

func TestEncodePCMToALaw(t *testing.T) {
	pcm := []int16{0, 100, -100, 5000, -5000}
	out := EncodePCMToALaw(pcm)
	if len(out) != len(pcm) {
		t.Fatalf("EncodePCMToALaw: got %d bytes, want %d", len(out), len(pcm))
	}
	for i, s := range pcm {
		want := LinearToALaw(s)
		if out[i] != want {
			t.Errorf("EncodePCMToALaw[%d]: got 0x%02x, want 0x%02x", i, out[i], want)
		}
	}
}

func TestEncodePCMEmpty(t *testing.T) {
	if out := EncodePCMToMuLaw(nil); len(out) != 0 {
		t.Errorf("EncodePCMToMuLaw(nil): got %d bytes, want 0", len(out))
	}
	if out := EncodePCMToALaw(nil); len(out) != 0 {
		t.Errorf("EncodePCMToALaw(nil): got %d bytes, want 0", len(out))
	}
}

func BenchmarkLinearToMuLaw(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LinearToMuLaw(int16(i % 65536))
	}
}

func BenchmarkLinearToALaw(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LinearToALaw(int16(i % 65536))
	}
}
