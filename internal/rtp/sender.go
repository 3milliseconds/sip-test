package rtp

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nightowl/sip-test/internal/media"
	"github.com/pion/rtp"
)

// Stats holds RTP session statistics.
type Stats struct {
	PacketsSent     uint64
	BytesSent       uint64
	PacketsReceived uint64
	BytesReceived   uint64
	StartTime       time.Time
	EndTime         time.Time
}

// Sender handles sending RTP audio packets over UDP.
type Sender struct {
	localAddr  string
	remoteAddr string
	conn       *net.UDPConn
	ssrc       uint32
	payloadType uint8
	clockRate  uint32

	mu      sync.Mutex
	running atomic.Bool
	stopCh  chan struct{}
	stats   Stats

	// For receiving RTP from remote
	recvBuf    []byte
	onReceive  func(pkt *rtp.Packet)
}

// NewSender creates a new RTP sender.
func NewSender(localAddr, remoteAddr string, codec media.CodecType) *Sender {
	var pt uint8
	switch codec {
	case media.CodecPCMU:
		pt = 0
	case media.CodecPCMA:
		pt = 8
	}

	return &Sender{
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		ssrc:        rand.Uint32(),
		payloadType: pt,
		clockRate:   8000,
		stopCh:      make(chan struct{}),
		recvBuf:     make([]byte, 1500),
	}
}

// SetOnReceive sets a callback for received RTP packets.
func (s *Sender) SetOnReceive(fn func(pkt *rtp.Packet)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onReceive = fn
}

// LocalPort returns the local UDP port after binding.
func (s *Sender) LocalPort() int {
	if s.conn == nil {
		return 0
	}
	return s.conn.LocalAddr().(*net.UDPAddr).Port
}

// Start binds the local UDP socket, starts the receive loop,
// and sends audio frames at 20ms intervals.
func (s *Sender) Start(source *media.AudioSource, repeat bool) error {
	localUDP, err := net.ResolveUDPAddr("udp", s.localAddr)
	if err != nil {
		return fmt.Errorf("resolve local addr: %w", err)
	}

	s.conn, err = net.ListenUDP("udp", localUDP)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}

	remoteUDP, err := net.ResolveUDPAddr("udp", s.remoteAddr)
	if err != nil {
		s.conn.Close()
		return fmt.Errorf("resolve remote addr: %w", err)
	}

	s.running.Store(true)
	s.stats.StartTime = time.Now()

	// Start receive goroutine
	go s.receiveLoop()

	// Start send goroutine
	go s.sendLoop(source, remoteUDP, repeat)

	return nil
}

// StartReceiveOnly binds the socket and only receives (no sending).
func (s *Sender) StartReceiveOnly() error {
	localUDP, err := net.ResolveUDPAddr("udp", s.localAddr)
	if err != nil {
		return fmt.Errorf("resolve local addr: %w", err)
	}

	s.conn, err = net.ListenUDP("udp", localUDP)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}

	s.running.Store(true)
	s.stats.StartTime = time.Now()

	go s.receiveLoop()
	return nil
}

func (s *Sender) sendLoop(source *media.AudioSource, remote *net.UDPAddr, repeat bool) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var seqNum uint16
	var timestamp uint32
	frameIdx := 0

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if frameIdx >= len(source.Frames) {
				if repeat {
					frameIdx = 0
				} else {
					return
				}
			}

			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Padding:        false,
					Extension:      false,
					Marker:         frameIdx == 0,
					PayloadType:    s.payloadType,
					SequenceNumber: seqNum,
					Timestamp:      timestamp,
					SSRC:           s.ssrc,
				},
				Payload: source.Frames[frameIdx],
			}

			data, err := pkt.Marshal()
			if err != nil {
				continue
			}

			_, err = s.conn.WriteToUDP(data, remote)
			if err != nil {
				continue
			}

			atomic.AddUint64(&s.stats.PacketsSent, 1)
			atomic.AddUint64(&s.stats.BytesSent, uint64(len(data)))

			seqNum++
			timestamp += 160 // 20ms at 8kHz
			frameIdx++
		}
	}
}

func (s *Sender) receiveLoop() {
	for s.running.Load() {
		s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := s.conn.ReadFromUDP(s.recvBuf)
		if err != nil {
			continue
		}

		pkt := &rtp.Packet{}
		if err := pkt.Unmarshal(s.recvBuf[:n]); err != nil {
			continue
		}

		atomic.AddUint64(&s.stats.PacketsReceived, 1)
		atomic.AddUint64(&s.stats.BytesReceived, uint64(n))

		s.mu.Lock()
		cb := s.onReceive
		s.mu.Unlock()
		if cb != nil {
			cb(pkt)
		}
	}
}

// Stop stops the RTP sender and closes the UDP socket.
func (s *Sender) Stop() {
	if !s.running.Load() {
		return
	}
	s.running.Store(false)
	close(s.stopCh)
	s.stats.EndTime = time.Now()
	if s.conn != nil {
		s.conn.Close()
	}
}

// GetStats returns a copy of the current RTP statistics.
func (s *Sender) GetStats() Stats {
	return Stats{
		PacketsSent:     atomic.LoadUint64(&s.stats.PacketsSent),
		BytesSent:       atomic.LoadUint64(&s.stats.BytesSent),
		PacketsReceived: atomic.LoadUint64(&s.stats.PacketsReceived),
		BytesReceived:   atomic.LoadUint64(&s.stats.BytesReceived),
		StartTime:       s.stats.StartTime,
		EndTime:         s.stats.EndTime,
	}
}
