package media

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
)

// CodecType represents the target codec for RTP.
type CodecType int

const (
	CodecPCMU CodecType = iota // G.711 mu-law (PT 0)
	CodecPCMA                  // G.711 A-law (PT 8)
)

// AudioSource holds decoded, resampled, and encoded audio ready for RTP.
type AudioSource struct {
	// Frames contains G.711-encoded audio split into 20ms frames.
	// Each frame is 160 bytes (8000 Hz * 0.020s = 160 samples).
	Frames [][]byte
	Codec  CodecType
}

const (
	targetSampleRate = 8000
	frameDuration    = 20 // ms
	samplesPerFrame  = targetSampleRate * frameDuration / 1000 // 160
)

// LoadAudio reads an audio file, decodes it, resamples to 8kHz mono,
// and encodes it to G.711 frames ready for RTP packetization.
func LoadAudio(filePath string, codec CodecType) (*AudioSource, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	var streamer beep.StreamSeekCloser
	var format beep.Format
	var err error

	switch ext {
	case ".mp3":
		f, ferr := os.Open(filePath)
		if ferr != nil {
			return nil, fmt.Errorf("open audio file: %w", ferr)
		}
		streamer, format, err = mp3.Decode(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("decode mp3: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}
	defer streamer.Close()

	// Resample to 8kHz mono
	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(targetSampleRate), streamer)

	// Read all samples into PCM buffer
	var pcmSamples []int16
	buf := make([][2]float64, 512)
	for {
		n, ok := resampled.Stream(buf)
		if n == 0 && !ok {
			break
		}
		for i := 0; i < n; i++ {
			// Mix stereo to mono
			mono := (buf[i][0] + buf[i][1]) / 2.0
			// Clamp and convert to int16
			if mono > 1.0 {
				mono = 1.0
			}
			if mono < -1.0 {
				mono = -1.0
			}
			pcmSamples = append(pcmSamples, int16(mono*32767))
		}
		if !ok {
			break
		}
	}

	if err := streamer.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	// Encode to G.711 and split into 20ms frames
	var encode func([]int16) []byte
	switch codec {
	case CodecPCMU:
		encode = EncodePCMToMuLaw
	case CodecPCMA:
		encode = EncodePCMToALaw
	}

	encoded := encode(pcmSamples)

	// Split into frames
	var frames [][]byte
	for i := 0; i+samplesPerFrame <= len(encoded); i += samplesPerFrame {
		frame := make([]byte, samplesPerFrame)
		copy(frame, encoded[i:i+samplesPerFrame])
		frames = append(frames, frame)
	}

	if len(frames) == 0 {
		return nil, fmt.Errorf("audio file too short to produce any frames")
	}

	return &AudioSource{
		Frames: frames,
		Codec:  codec,
	}, nil
}

// GenerateSilence creates silent G.711 frames for the given duration in milliseconds.
func GenerateSilence(durationMs int, codec CodecType) *AudioSource {
	numFrames := durationMs / frameDuration
	if numFrames == 0 {
		numFrames = 1
	}

	// Silence in mu-law is 0xFF, in A-law is 0xD5
	var silenceByte byte
	switch codec {
	case CodecPCMU:
		silenceByte = 0xFF
	case CodecPCMA:
		silenceByte = 0xD5
	}

	frames := make([][]byte, numFrames)
	for i := range frames {
		frame := make([]byte, samplesPerFrame)
		for j := range frame {
			frame[j] = silenceByte
		}
		frames[i] = frame
	}

	return &AudioSource{
		Frames: frames,
		Codec:  codec,
	}
}

// PCMToBytes converts int16 PCM samples to little-endian bytes.
func PCMToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}
