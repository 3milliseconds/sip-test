package testrunner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nightowl/sip-test/internal/media"
	"github.com/nightowl/sip-test/internal/report"
	"github.com/nightowl/sip-test/internal/rtp"
	sipClient "github.com/nightowl/sip-test/internal/sip"
)

// RunStatus tracks the status of a running test.
type RunStatus struct {
	TestName  string `json:"test_name"`
	State     string `json:"state"`
	StartedAt string `json:"started_at"`
	Error     string `json:"error,omitempty"`
}

// Runner orchestrates SIP test execution.
type Runner struct {
	mu       sync.Mutex
	running  map[string]*runContext
	results  []*report.TestReport
	onUpdate func(status RunStatus) // callback for status updates
}

type runContext struct {
	cancel context.CancelFunc
	status RunStatus
}

// NewRunner creates a new test runner.
func NewRunner() *Runner {
	return &Runner{
		running: make(map[string]*runContext),
	}
}

// SetOnUpdate sets a callback for test status updates.
func (r *Runner) SetOnUpdate(fn func(status RunStatus)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUpdate = fn
}

func (r *Runner) notify(status RunStatus) {
	r.mu.Lock()
	fn := r.onUpdate
	r.mu.Unlock()
	if fn != nil {
		fn(status)
	}
}

// RunTest executes a single test case and returns a report.
func (r *Runner) RunTest(ctx context.Context, tc TestConfig) (*report.TestReport, error) {
	tr := &report.TestReport{
		TestName:  tc.Name,
		Status:    "running",
		Codec:     tc.Codec,
		StartTime: time.Now(),
	}

	// Track running test
	testCtx, cancel := context.WithTimeout(ctx, tc.Duration+30*time.Second)
	r.mu.Lock()
	r.running[tc.Name] = &runContext{
		cancel: cancel,
		status: RunStatus{TestName: tc.Name, State: "starting", StartedAt: time.Now().Format(time.RFC3339)},
	}
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.running, tc.Name)
		r.mu.Unlock()
	}()

	r.notify(RunStatus{TestName: tc.Name, State: "starting"})

	// Resolve codec
	codec, err := ParseCodec(tc.Codec)
	if err != nil {
		tr.Status = "error"
		tr.Error = err.Error()
		return tr, err
	}

	// Get local IP
	localIP, err := sipClient.GetLocalIP()
	if err != nil {
		tr.Status = "error"
		tr.Error = fmt.Sprintf("get local IP: %v", err)
		return tr, err
	}

	if tc.From == "" {
		tc.From = fmt.Sprintf("sip:sip-test@%s", localIP)
	}

	// Load audio source
	var audioSource *media.AudioSource
	if tc.MediaFile != "" {
		r.notify(RunStatus{TestName: tc.Name, State: "loading_audio"})
		audioSource, err = media.LoadAudio(tc.MediaFile, codec)
		if err != nil {
			tr.Status = "error"
			tr.Error = fmt.Sprintf("load audio: %v", err)
			return tr, err
		}
	} else {
		// Generate silence if no media file
		audioSource = media.GenerateSilence(int(tc.Duration.Milliseconds()), codec)
	}

	// Create RTP sender (bind to auto port if 0)
	rtpLocalAddr := fmt.Sprintf("%s:%d", localIP, tc.RTPPort)
	// We'll set remote addr after SDP negotiation
	rtpSender := rtp.NewSender(rtpLocalAddr, "0.0.0.0:0", codec)

	// Bind RTP socket to get port for SDP
	if err := rtpSender.StartReceiveOnly(); err != nil {
		tr.Status = "error"
		tr.Error = fmt.Sprintf("bind RTP: %v", err)
		return tr, err
	}
	rtpPort := rtpSender.LocalPort()
	rtpSender.Stop()

	// Create SIP client
	r.notify(RunStatus{TestName: tc.Name, State: "sip_invite"})
	sipCli, err := sipClient.NewClient(localIP, tc.SIPPort, tc.Transport)
	if err != nil {
		tr.Status = "error"
		tr.Error = fmt.Sprintf("create SIP client: %v", err)
		return tr, err
	}
	defer sipCli.Close()

	// Send INVITE
	inviteCtx, inviteCancel := context.WithTimeout(testCtx, tc.Expected.SetupTimeMax)
	defer inviteCancel()

	callResult, err := sipCli.Invite(inviteCtx, tc.Target, tc.From, rtpPort, codec)
	if err != nil {
		tr.Status = "failed"
		tr.Error = fmt.Sprintf("INVITE failed: %v", err)
		tr.SIPFlow = convertEvents(callResult.Events)
		return tr, nil
	}

	tr.CallSetupTimeMs = float64(callResult.SetupTime.Milliseconds())
	tr.SIPFlow = convertEvents(callResult.Events)

	// Check expected response
	if callResult.State != sipClient.StateActive {
		tr.Status = "failed"
		tr.Error = fmt.Sprintf("call not established, state: %s", callResult.State)
		if callResult.Error != nil {
			tr.Error += ": " + callResult.Error.Error()
		}
		return tr, nil
	}

	// Start RTP streaming
	r.notify(RunStatus{TestName: tc.Name, State: "rtp_streaming"})
	remoteRTPAddr := callResult.RemoteRTPAddr
	rtpSender2 := rtp.NewSender(
		fmt.Sprintf("%s:%d", localIP, rtpPort),
		remoteRTPAddr,
		codec,
	)

	if err := rtpSender2.Start(audioSource, tc.RepeatMedia); err != nil {
		tr.Status = "error"
		tr.Error = fmt.Sprintf("start RTP: %v", err)
		return tr, err
	}

	// Wait for call duration
	log.Printf("[%s] Call active, streaming RTP for %v", tc.Name, tc.Duration)
	select {
	case <-testCtx.Done():
	case <-time.After(tc.Duration):
	}

	// Stop RTP
	rtpSender2.Stop()
	rtpStats := rtpSender2.GetStats()

	// Send BYE
	r.notify(RunStatus{TestName: tc.Name, State: "sip_bye"})
	byeCtx, byeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer byeCancel()

	byeEvent, err := sipCli.SendBye(byeCtx, tc.Target, tc.From, "", "", "")
	if byeEvent != nil {
		tr.SIPFlow = append(tr.SIPFlow, report.SIPFlowEvent{
			Direction: byeEvent.Direction,
			Method:    byeEvent.Method,
			Timestamp: byeEvent.Timestamp.Format(time.RFC3339Nano),
		})
	}
	if err != nil {
		log.Printf("[%s] BYE error (non-fatal): %v", tc.Name, err)
	}

	// Build report
	tr.Status = "passed"
	tr.RTPPacketsSent = rtpStats.PacketsSent
	tr.RTPPacketsReceived = rtpStats.PacketsReceived
	tr.CallDurationS = tc.Duration.Seconds()
	tr.EndTime = time.Now()

	if rtpStats.PacketsSent > 0 && rtpStats.PacketsReceived > 0 {
		loss := float64(rtpStats.PacketsSent-rtpStats.PacketsReceived) / float64(rtpStats.PacketsSent) * 100
		if loss < 0 {
			loss = 0
		}
		tr.PacketLossPct = loss
	}

	// Validate expectations
	if tc.Expected.SetupTimeMax > 0 && callResult.SetupTime > tc.Expected.SetupTimeMax {
		tr.Status = "failed"
		tr.Error = fmt.Sprintf("setup time %v exceeded max %v", callResult.SetupTime, tc.Expected.SetupTimeMax)
	}

	r.mu.Lock()
	r.results = append(r.results, tr)
	r.mu.Unlock()

	r.notify(RunStatus{TestName: tc.Name, State: tr.Status})
	return tr, nil
}

// RunAll executes all tests in a config sequentially and returns all reports.
func (r *Runner) RunAll(ctx context.Context, cfg *Config) ([]*report.TestReport, error) {
	var reports []*report.TestReport
	for _, tc := range cfg.Tests {
		tr, err := r.RunTest(ctx, tc)
		if err != nil {
			log.Printf("Test %q error: %v", tc.Name, err)
		}
		if tr != nil {
			reports = append(reports, tr)
		}
	}
	return reports, nil
}

// GetResults returns all completed test results.
func (r *Runner) GetResults() []*report.TestReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*report.TestReport, len(r.results))
	copy(out, r.results)
	return out
}

// GetRunning returns status of all currently running tests.
func (r *Runner) GetRunning() []RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	var statuses []RunStatus
	for _, rc := range r.running {
		statuses = append(statuses, rc.status)
	}
	return statuses
}

func convertEvents(events []sipClient.SIPEvent) []report.SIPFlowEvent {
	var out []report.SIPFlowEvent
	for _, e := range events {
		out = append(out, report.SIPFlowEvent{
			Direction: e.Direction,
			Method:    e.Method,
			Status:    e.Status,
			Timestamp: e.Timestamp.Format(time.RFC3339Nano),
		})
	}
	return out
}
