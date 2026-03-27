package media

import (
	"encoding/binary"
	"testing"
)

func TestGenerateSilencePCMU(t *testing.T) {
	src := GenerateSilence(100, CodecPCMU) // 100ms = 5 frames of 20ms
	if src == nil {
		t.Fatal("GenerateSilence returned nil")
	}
	if src.Codec != CodecPCMU {
		t.Errorf("codec: got %d, want %d (CodecPCMU)", src.Codec, CodecPCMU)
	}
	if len(src.Frames) != 5 {
		t.Errorf("frames: got %d, want 5", len(src.Frames))
	}
	for i, frame := range src.Frames {
		if len(frame) != 160 {
			t.Errorf("frame[%d] size: got %d, want 160", i, len(frame))
		}
		for j, b := range frame {
			if b != 0xFF {
				t.Errorf("frame[%d][%d]: got 0x%02x, want 0xFF (mu-law silence)", i, j, b)
				break
			}
		}
	}
}

func TestGenerateSilencePCMA(t *testing.T) {
	src := GenerateSilence(60, CodecPCMA) // 60ms = 3 frames
	if src == nil {
		t.Fatal("GenerateSilence returned nil")
	}
	if src.Codec != CodecPCMA {
		t.Errorf("codec: got %d, want %d (CodecPCMA)", src.Codec, CodecPCMA)
	}
	if len(src.Frames) != 3 {
		t.Errorf("frames: got %d, want 3", len(src.Frames))
	}
	for i, frame := range src.Frames {
		if len(frame) != 160 {
			t.Errorf("frame[%d] size: got %d, want 160", i, len(frame))
		}
		for j, b := range frame {
			if b != 0xD5 {
				t.Errorf("frame[%d][%d]: got 0x%02x, want 0xD5 (A-law silence)", i, j, b)
				break
			}
		}
	}
}

func TestGenerateSilenceMinOneFrame(t *testing.T) {
	// Duration shorter than one frame should still produce at least 1 frame
	src := GenerateSilence(5, CodecPCMU)
	if len(src.Frames) != 1 {
		t.Errorf("frames: got %d, want 1 (minimum)", len(src.Frames))
	}
}

func TestGenerateSilenceZeroDuration(t *testing.T) {
	src := GenerateSilence(0, CodecPCMU)
	if len(src.Frames) < 1 {
		t.Error("0ms duration should produce at least 1 frame")
	}
}

func TestPCMToBytes(t *testing.T) {
	samples := []int16{0, 1, -1, 256, -256, 32767, -32768}
	buf := PCMToBytes(samples)
	if len(buf) != len(samples)*2 {
		t.Fatalf("PCMToBytes: got %d bytes, want %d", len(buf), len(samples)*2)
	}
	for i, s := range samples {
		got := int16(binary.LittleEndian.Uint16(buf[i*2:]))
		if got != s {
			t.Errorf("PCMToBytes[%d]: got %d, want %d", i, got, s)
		}
	}
}

func TestPCMToBytesEmpty(t *testing.T) {
	buf := PCMToBytes(nil)
	if len(buf) != 0 {
		t.Errorf("PCMToBytes(nil): got %d bytes, want 0", len(buf))
	}
}

func TestLoadAudioUnsupportedFormat(t *testing.T) {
	_, err := LoadAudio("test.wav", CodecPCMU)
	if err == nil {
		t.Error("LoadAudio should reject unsupported format")
	}
}

func TestLoadAudioNonexistentFile(t *testing.T) {
	_, err := LoadAudio("nonexistent.mp3", CodecPCMU)
	if err == nil {
		t.Error("LoadAudio should fail on nonexistent file")
	}
}
