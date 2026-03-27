package rtp

import (
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nightowl/sip-test/internal/media"
	"github.com/pion/rtp"
)

func TestNewSender(t *testing.T) {
	s := NewSender("127.0.0.1:0", "127.0.0.1:9999", media.CodecPCMU)
	if s.payloadType != 0 {
		t.Errorf("PCMU payload type: got %d, want 0", s.payloadType)
	}
	if s.clockRate != 8000 {
		t.Errorf("clock rate: got %d, want 8000", s.clockRate)
	}

	s2 := NewSender("127.0.0.1:0", "127.0.0.1:9999", media.CodecPCMA)
	if s2.payloadType != 8 {
		t.Errorf("PCMA payload type: got %d, want 8", s2.payloadType)
	}
}

func TestSenderStartReceiveOnly(t *testing.T) {
	s := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	if err := s.StartReceiveOnly(); err != nil {
		t.Fatalf("StartReceiveOnly: %v", err)
	}
	defer s.Stop()

	if !s.running.Load() {
		t.Error("sender should be running after StartReceiveOnly")
	}
	port := s.LocalPort()
	if port == 0 {
		t.Error("LocalPort should be non-zero after binding")
	}
}

func TestSenderLocalPortBeforeBind(t *testing.T) {
	s := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	if port := s.LocalPort(); port != 0 {
		t.Errorf("LocalPort before bind: got %d, want 0", port)
	}
}

func TestSenderStopIdempotent(t *testing.T) {
	s := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	if err := s.StartReceiveOnly(); err != nil {
		t.Fatalf("StartReceiveOnly: %v", err)
	}
	s.Stop()
	// Second stop should not panic
	s.Stop()
}

func TestSenderSendAndReceive(t *testing.T) {
	// Create a receiver
	recvAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	recvConn, err := net.ListenUDP("udp", recvAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer recvConn.Close()
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port

	// Create sender targeting the receiver
	audio := media.GenerateSilence(100, media.CodecPCMU) // 5 frames
	sender := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	sender.remoteAddr = net.JoinHostPort("127.0.0.1", itoa(recvPort))

	if err := sender.Start(audio, false); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sender.Stop()

	// Read at least one packet from the receiver
	buf := make([]byte, 1500)
	recvConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := recvConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	pkt := &rtp.Packet{}
	if err := pkt.Unmarshal(buf[:n]); err != nil {
		t.Fatalf("unmarshal RTP: %v", err)
	}

	if pkt.Header.Version != 2 {
		t.Errorf("RTP version: got %d, want 2", pkt.Header.Version)
	}
	if pkt.Header.PayloadType != 0 {
		t.Errorf("payload type: got %d, want 0 (PCMU)", pkt.Header.PayloadType)
	}
	if len(pkt.Payload) != 160 {
		t.Errorf("payload size: got %d, want 160", len(pkt.Payload))
	}
	if pkt.Header.SSRC != sender.ssrc {
		t.Errorf("SSRC mismatch: got %d, want %d", pkt.Header.SSRC, sender.ssrc)
	}
}

func TestSenderStats(t *testing.T) {
	recvAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	recvConn, err := net.ListenUDP("udp", recvAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer recvConn.Close()
	recvPort := recvConn.LocalAddr().(*net.UDPAddr).Port

	audio := media.GenerateSilence(60, media.CodecPCMU) // 3 frames
	sender := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	sender.remoteAddr = net.JoinHostPort("127.0.0.1", itoa(recvPort))

	if err := sender.Start(audio, false); err != nil {
		t.Fatal(err)
	}

	// Wait for frames to be sent (3 frames * 20ms + margin)
	time.Sleep(120 * time.Millisecond)
	sender.Stop()

	stats := sender.GetStats()
	if stats.PacketsSent == 0 {
		t.Error("PacketsSent should be > 0")
	}
	if stats.BytesSent == 0 {
		t.Error("BytesSent should be > 0")
	}
	if stats.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
	if stats.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
}

func TestSenderOnReceive(t *testing.T) {
	// Create sender in receive-only mode
	sender := NewSender("127.0.0.1:0", "127.0.0.1:0", media.CodecPCMU)
	if err := sender.StartReceiveOnly(); err != nil {
		t.Fatal(err)
	}
	defer sender.Stop()

	var received atomic.Int32
	sender.SetOnReceive(func(pkt *rtp.Packet) {
		received.Add(1)
	})

	// Send a packet to the sender's port
	senderPort := sender.LocalPort()
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: senderPort})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:     2,
			PayloadType: 0,
			SSRC:        12345,
		},
		Payload: make([]byte, 160),
	}
	data, _ := pkt.Marshal()
	conn.Write(data)

	// Wait for receive
	time.Sleep(200 * time.Millisecond)
	if received.Load() == 0 {
		t.Error("onReceive callback should have been called")
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
