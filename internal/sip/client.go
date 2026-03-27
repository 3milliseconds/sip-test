package sip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/nightowl/sip-test/internal/media"
)

// CallState represents the state of a SIP call.
type CallState int

const (
	StateIdle CallState = iota
	StateInviteSent
	StateTrying
	StateRinging
	StateAnswered
	StateActive
	StateBye
	StateFailed
	StateTimeout
)

func (s CallState) String() string {
	names := []string{"idle", "invite_sent", "trying", "ringing", "answered", "active", "bye", "failed", "timeout"}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

// SIPEvent records a SIP signaling event for the test report.
type SIPEvent struct {
	Direction string    `json:"direction"` // "sent" or "recv"
	Method    string    `json:"method,omitempty"`
	Status    int       `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// CallResult holds the outcome of a SIP call attempt.
type CallResult struct {
	State         CallState
	RemoteRTPAddr string // ip:port for RTP
	RemoteRTPIP   string
	RemoteRTPPort int
	SetupTime     time.Duration
	Events        []SIPEvent
	Error         error
}

// Client is a SIP UAC (User Agent Client) for making test calls.
type Client struct {
	ua        *sipgo.UserAgent
	client    *sipgo.Client
	localIP   string
	localPort int
	transport string

	mu    sync.Mutex
	state CallState
}

// NewClient creates a new SIP client.
func NewClient(localIP string, localPort int, transport string) (*Client, error) {
	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent("sip-test/1.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("create user agent: %w", err)
	}

	client, err := sipgo.NewClient(ua)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return &Client{
		ua:        ua,
		client:    client,
		localIP:   localIP,
		localPort: localPort,
		transport: transport,
		state:     StateIdle,
	}, nil
}

// Invite sends a SIP INVITE and waits for a final response.
// Returns the call result with remote RTP info on success.
func (c *Client) Invite(ctx context.Context, target, from string, rtpPort int, codec media.CodecType) (*CallResult, error) {
	result := &CallResult{
		State: StateIdle,
	}

	startTime := time.Now()

	// Build SDP offer
	sdpBody := BuildOfferSDP(c.localIP, rtpPort, codec)

	// Build INVITE request
	req := sip.NewRequest(sip.INVITE, sip.Uri{})
	if err := sip.ParseUri(target, &req.Recipient); err != nil {
		return nil, fmt.Errorf("parse target URI: %w", err)
	}

	// Set headers
	from = fmt.Sprintf("<%s>", from)
	req.AppendHeader(sip.NewHeader("From", fmt.Sprintf("%s;tag=%s", from, sip.GenerateTagN(16))))
	req.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<%s>", target)))
	contactURI := fmt.Sprintf("<sip:%s:%d;transport=%s>", c.localIP, c.localPort, c.transport)
	req.AppendHeader(sip.NewHeader("Contact", contactURI))
	req.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	req.AppendHeader(sip.NewHeader("Allow", "INVITE, ACK, CANCEL, BYE"))
	req.AppendHeader(sip.NewHeader("Max-Forwards", "70"))
	req.SetBody([]byte(sdpBody))

	result.Events = append(result.Events, SIPEvent{
		Direction: "sent",
		Method:    "INVITE",
		Timestamp: time.Now(),
	})

	c.mu.Lock()
	c.state = StateInviteSent
	c.mu.Unlock()

	// Send INVITE via client transaction
	tx, err := c.client.TransactionRequest(ctx, req)
	if err != nil {
		result.State = StateFailed
		result.Error = err
		return result, fmt.Errorf("send INVITE: %w", err)
	}
	defer tx.Terminate()

	// Wait for responses
	for {
		select {
		case <-ctx.Done():
			result.State = StateTimeout
			result.Error = ctx.Err()
			return result, nil

		case res, ok := <-tx.Responses():
			if !ok {
				if result.State < StateAnswered {
					result.State = StateFailed
					result.Error = fmt.Errorf("transaction closed without final response")
				}
				return result, nil
			}

			statusCode := int(res.StatusCode)
			result.Events = append(result.Events, SIPEvent{
				Direction: "recv",
				Status:    statusCode,
				Timestamp: time.Now(),
			})

			switch {
			case statusCode == 100:
				c.mu.Lock()
				c.state = StateTrying
				c.mu.Unlock()

			case statusCode >= 180 && statusCode < 200:
				c.mu.Lock()
				c.state = StateRinging
				c.mu.Unlock()

			case statusCode == 200:
				c.mu.Lock()
				c.state = StateAnswered
				c.mu.Unlock()
				result.State = StateAnswered
				result.SetupTime = time.Since(startTime)

				// Parse SDP answer
				body := string(res.Body())
				if body != "" {
					rIP, rPort, err := ParseAnswerSDP(ExtractSDPBody(body))
					if err != nil {
						result.Error = fmt.Errorf("parse answer SDP: %w", err)
						result.State = StateFailed
						return result, nil
					}
					result.RemoteRTPIP = rIP
					result.RemoteRTPPort = rPort
					result.RemoteRTPAddr = fmt.Sprintf("%s:%d", rIP, rPort)
				}

				// Send ACK - construct manually
				ack := buildACK(req, res, c.localIP, c.localPort, c.transport)
				if err := c.client.WriteRequest(ack); err != nil {
					result.Error = fmt.Errorf("send ACK: %w", err)
					return result, nil
				}
				result.Events = append(result.Events, SIPEvent{
					Direction: "sent",
					Method:    "ACK",
					Timestamp: time.Now(),
				})

				c.mu.Lock()
				c.state = StateActive
				c.mu.Unlock()
				result.State = StateActive
				return result, nil

			case statusCode >= 300:
				result.State = StateFailed
				result.Error = fmt.Errorf("call rejected with status %d", statusCode)
				return result, nil
			}
		}
	}
}

// SendBye sends a BYE request to terminate a call.
func (c *Client) SendBye(ctx context.Context, target, from, toTag, fromTag, callID string) (*SIPEvent, error) {
	req := sip.NewRequest(sip.BYE, sip.Uri{})
	if err := sip.ParseUri(target, &req.Recipient); err != nil {
		return nil, fmt.Errorf("parse target URI: %w", err)
	}

	req.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<%s>;tag=%s", from, fromTag)))
	req.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<%s>;tag=%s", target, toTag)))
	req.AppendHeader(sip.NewHeader("Call-ID", callID))
	contactURI := fmt.Sprintf("<sip:%s:%d;transport=%s>", c.localIP, c.localPort, c.transport)
	req.AppendHeader(sip.NewHeader("Contact", contactURI))

	event := &SIPEvent{
		Direction: "sent",
		Method:    "BYE",
		Timestamp: time.Now(),
	}

	tx, err := c.client.TransactionRequest(ctx, req)
	if err != nil {
		return event, fmt.Errorf("send BYE: %w", err)
	}
	defer tx.Terminate()

	// Wait for 200 OK response to BYE
	select {
	case <-ctx.Done():
		return event, ctx.Err()
	case res, ok := <-tx.Responses():
		if ok && res.StatusCode == 200 {
			c.mu.Lock()
			c.state = StateBye
			c.mu.Unlock()
		}
	}

	return event, nil
}

// State returns the current call state.
func (c *Client) State() CallState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Close cleans up the client resources.
func (c *Client) Close() error {
	return nil
}

// buildACK constructs an ACK request from the original INVITE request and 200 OK response.
func buildACK(invite *sip.Request, response *sip.Response, localIP string, localPort int, transport string) *sip.Request {
	ack := sip.NewRequest(sip.ACK, invite.Recipient)

	// Copy Via, From, To (with tag from response), Call-ID, CSeq from the INVITE
	if fromH := invite.From(); fromH != nil {
		ack.AppendHeader(sip.HeaderClone(fromH))
	}
	// To header from response includes the remote tag
	if toH := response.To(); toH != nil {
		ack.AppendHeader(sip.HeaderClone(toH))
	}
	if callID := invite.CallID(); callID != nil {
		ack.AppendHeader(sip.HeaderClone(callID))
	}
	// CSeq must match INVITE but with ACK method
	ack.AppendHeader(sip.NewHeader("CSeq", fmt.Sprintf("%d ACK", invite.CSeq().SeqNo)))
	contactURI := fmt.Sprintf("<sip:%s:%d;transport=%s>", localIP, localPort, transport)
	ack.AppendHeader(sip.NewHeader("Contact", contactURI))
	ack.AppendHeader(sip.NewHeader("Max-Forwards", "70"))

	return ack
}
