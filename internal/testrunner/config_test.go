package testrunner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nightowl/sip-test/internal/media"
)

func TestParseCodecValid(t *testing.T) {
	tests := []struct {
		input string
		want  media.CodecType
	}{
		{"PCMU", media.CodecPCMU},
		{"pcmu", media.CodecPCMU},
		{"G711U", media.CodecPCMU},
		{"ULAW", media.CodecPCMU},
		{"ulaw", media.CodecPCMU},
		{"PCMA", media.CodecPCMA},
		{"pcma", media.CodecPCMA},
		{"G711A", media.CodecPCMA},
		{"ALAW", media.CodecPCMA},
		{"alaw", media.CodecPCMA},
	}
	for _, tt := range tests {
		got, err := ParseCodec(tt.input)
		if err != nil {
			t.Errorf("ParseCodec(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseCodec(%q): got %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseCodecInvalid(t *testing.T) {
	invalids := []string{"opus", "G729", "MP3", ""}
	for _, codec := range invalids {
		_, err := ParseCodec(codec)
		if err == nil {
			t.Errorf("ParseCodec(%q): expected error, got nil", codec)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	yaml := `tests:
  - name: "Test 1"
    target: "sip:1000@10.0.0.1:5060"
    from: "sip:tester@10.0.0.2"
    transport: tcp
    codec: PCMA
    duration: 60s
    media_file: "/tmp/audio.mp3"
    repeat_media: true
    rtp_port: 5004
    sip_port: 5080
    expected:
      response_code: 200
      setup_time_max: 5s
  - name: "Test 2"
    target: "sip:2000@10.0.0.1:5060"
`

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg.Tests) != 2 {
		t.Fatalf("tests: got %d, want 2", len(cfg.Tests))
	}

	// Test 1: explicit values
	tc := cfg.Tests[0]
	if tc.Name != "Test 1" {
		t.Errorf("name: got %q", tc.Name)
	}
	if tc.Transport != "tcp" {
		t.Errorf("transport: got %q, want tcp", tc.Transport)
	}
	if tc.Codec != "PCMA" {
		t.Errorf("codec: got %q, want PCMA", tc.Codec)
	}
	if tc.Duration != 60*time.Second {
		t.Errorf("duration: got %v, want 60s", tc.Duration)
	}
	if tc.SIPPort != 5080 {
		t.Errorf("sip_port: got %d, want 5080", tc.SIPPort)
	}
	if tc.RTPPort != 5004 {
		t.Errorf("rtp_port: got %d, want 5004", tc.RTPPort)
	}
	if tc.Expected.ResponseCode != 200 {
		t.Errorf("response_code: got %d, want 200", tc.Expected.ResponseCode)
	}
	if tc.Expected.SetupTimeMax != 5*time.Second {
		t.Errorf("setup_time_max: got %v, want 5s", tc.Expected.SetupTimeMax)
	}

	// Test 2: defaults applied
	tc2 := cfg.Tests[1]
	if tc2.Transport != "udp" {
		t.Errorf("default transport: got %q, want udp", tc2.Transport)
	}
	if tc2.Codec != "PCMU" {
		t.Errorf("default codec: got %q, want PCMU", tc2.Codec)
	}
	if tc2.Duration != 30*time.Second {
		t.Errorf("default duration: got %v, want 30s", tc2.Duration)
	}
	if tc2.SIPPort != 5060 {
		t.Errorf("default sip_port: got %d, want 5060", tc2.SIPPort)
	}
	if tc2.Expected.ResponseCode != 200 {
		t.Errorf("default response_code: got %d, want 200", tc2.Expected.ResponseCode)
	}
	if tc2.Expected.SetupTimeMax != 10*time.Second {
		t.Errorf("default setup_time_max: got %v, want 10s", tc2.Expected.SetupTimeMax)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("LoadConfig should fail for missing file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Error("LoadConfig should fail for invalid YAML")
	}
}

func TestLoadConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(cfgPath, []byte("tests: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Tests) != 0 {
		t.Errorf("tests: got %d, want 0", len(cfg.Tests))
	}
}
