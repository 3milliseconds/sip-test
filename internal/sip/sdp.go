package sip

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"github.com/nightowl/sip-test/internal/media"
	"github.com/pion/sdp/v3"
)

// BuildOfferSDP creates an SDP offer for an audio call.
func BuildOfferSDP(localIP string, rtpPort int, codec media.CodecType) string {
	sessID := rand.Int63()

	var ptNum uint8
	var codecName string
	switch codec {
	case media.CodecPCMU:
		ptNum = 0
		codecName = "PCMU"
	case media.CodecPCMA:
		ptNum = 8
		codecName = "PCMA"
	}

	sd := &sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      uint64(sessID),
			SessionVersion: uint64(sessID),
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: localIP,
		},
		SessionName: "sip-test",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: localIP},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{Timing: sdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media:   "audio",
					Port:    sdp.RangedPort{Value: rtpPort},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{strconv.Itoa(int(ptNum)), "101"},
				},
				Attributes: []sdp.Attribute{
					{Key: "rtpmap", Value: fmt.Sprintf("%d %s/8000", ptNum, codecName)},
					{Key: "rtpmap", Value: "101 telephone-event/8000"},
					{Key: "fmtp", Value: "101 0-16"},
					{Key: "sendrecv"},
					{Key: "ptime", Value: "20"},
				},
			},
		},
	}

	b, _ := sd.Marshal()
	return string(b)
}

// ParseAnswerSDP extracts the remote RTP address and port from an SDP answer.
func ParseAnswerSDP(body string) (remoteIP string, remotePort int, err error) {
	sd := &sdp.SessionDescription{}
	if err := sd.UnmarshalString(body); err != nil {
		return "", 0, fmt.Errorf("parse SDP: %w", err)
	}

	// Get connection info IP
	if sd.ConnectionInformation != nil && sd.ConnectionInformation.Address != nil {
		remoteIP = sd.ConnectionInformation.Address.Address
	}

	for _, m := range sd.MediaDescriptions {
		if m.MediaName.Media == "audio" {
			remotePort = m.MediaName.Port.Value
			// Media-level connection info overrides session-level
			if m.ConnectionInformation != nil && m.ConnectionInformation.Address != nil {
				remoteIP = m.ConnectionInformation.Address.Address
			}
			break
		}
	}

	if remoteIP == "" || remotePort == 0 {
		return "", 0, fmt.Errorf("no audio media in SDP answer")
	}

	return remoteIP, remotePort, nil
}

// GetLocalIP returns the preferred outbound IP address.
func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

// ExtractSDPBody extracts the SDP body from a SIP message body string.
// It trims any leading/trailing whitespace.
func ExtractSDPBody(body string) string {
	return strings.TrimSpace(body)
}
