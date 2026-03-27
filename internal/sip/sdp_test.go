package sip

import (
	"strings"
	"testing"

	"github.com/nightowl/sip-test/internal/media"
)

func TestBuildOfferSDPPCMU(t *testing.T) {
	sdp := BuildOfferSDP("192.168.1.100", 10000, media.CodecPCMU)

	checks := []struct {
		contains string
		desc     string
	}{
		{"v=0", "version line"},
		{"o=-", "origin line"},
		{"s=sip-test", "session name"},
		{"c=IN IP4 192.168.1.100", "connection info"},
		{"m=audio 10000 RTP/AVP 0 101", "media line with PCMU PT"},
		{"a=rtpmap:0 PCMU/8000", "PCMU rtpmap"},
		{"a=rtpmap:101 telephone-event/8000", "telephone-event rtpmap"},
		{"a=fmtp:101 0-16", "DTMF fmtp"},
		{"a=sendrecv", "sendrecv attribute"},
		{"a=ptime:20", "ptime attribute"},
	}

	for _, c := range checks {
		if !strings.Contains(sdp, c.contains) {
			t.Errorf("SDP missing %s: %q not found in:\n%s", c.desc, c.contains, sdp)
		}
	}
}

func TestBuildOfferSDPPCMA(t *testing.T) {
	sdp := BuildOfferSDP("10.0.0.1", 20000, media.CodecPCMA)

	if !strings.Contains(sdp, "m=audio 20000 RTP/AVP 8 101") {
		t.Errorf("SDP missing PCMA media line in:\n%s", sdp)
	}
	if !strings.Contains(sdp, "a=rtpmap:8 PCMA/8000") {
		t.Errorf("SDP missing PCMA rtpmap in:\n%s", sdp)
	}
	if !strings.Contains(sdp, "c=IN IP4 10.0.0.1") {
		t.Errorf("SDP missing connection info in:\n%s", sdp)
	}
}

func TestParseAnswerSDP(t *testing.T) {
	body := `v=0
o=- 12345 12345 IN IP4 10.0.0.50
s=call
c=IN IP4 10.0.0.50
t=0 0
m=audio 30000 RTP/AVP 0
a=rtpmap:0 PCMU/8000
a=ptime:20
`
	ip, port, err := ParseAnswerSDP(body)
	if err != nil {
		t.Fatalf("ParseAnswerSDP: %v", err)
	}
	if ip != "10.0.0.50" {
		t.Errorf("IP: got %q, want %q", ip, "10.0.0.50")
	}
	if port != 30000 {
		t.Errorf("port: got %d, want 30000", port)
	}
}

func TestParseAnswerSDPMediaLevelConnection(t *testing.T) {
	// Media-level c= should override session-level
	body := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=test
c=IN IP4 10.0.0.1
t=0 0
m=audio 40000 RTP/AVP 8
c=IN IP4 10.0.0.99
a=rtpmap:8 PCMA/8000
`
	ip, port, err := ParseAnswerSDP(body)
	if err != nil {
		t.Fatalf("ParseAnswerSDP: %v", err)
	}
	if ip != "10.0.0.99" {
		t.Errorf("IP: got %q, want %q (media-level override)", ip, "10.0.0.99")
	}
	if port != 40000 {
		t.Errorf("port: got %d, want 40000", port)
	}
}

func TestParseAnswerSDPNoAudio(t *testing.T) {
	body := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=test
c=IN IP4 10.0.0.1
t=0 0
m=video 50000 RTP/AVP 96
a=rtpmap:96 H264/90000
`
	_, _, err := ParseAnswerSDP(body)
	if err == nil {
		t.Error("ParseAnswerSDP should fail when no audio media present")
	}
}

func TestParseAnswerSDPInvalid(t *testing.T) {
	_, _, err := ParseAnswerSDP("this is not SDP")
	if err == nil {
		t.Error("ParseAnswerSDP should fail on invalid SDP")
	}
}

func TestExtractSDPBody(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  v=0\r\n  ", "v=0"},
		{"\n\nsome body\n\n", "some body"},
		{"clean", "clean"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ExtractSDPBody(tt.input)
		if got != tt.want {
			t.Errorf("ExtractSDPBody(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildAndParseRoundtrip(t *testing.T) {
	// Build an offer SDP then parse it as an answer to verify consistency
	sdpStr := BuildOfferSDP("172.16.0.1", 5004, media.CodecPCMU)
	ip, port, err := ParseAnswerSDP(sdpStr)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}
	if ip != "172.16.0.1" {
		t.Errorf("roundtrip IP: got %q, want %q", ip, "172.16.0.1")
	}
	if port != 5004 {
		t.Errorf("roundtrip port: got %d, want 5004", port)
	}
}
