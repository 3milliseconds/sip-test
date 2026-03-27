package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// TestReport holds the complete results of a single test.
type TestReport struct {
	TestName          string         `json:"test_name"`
	Status            string         `json:"status"` // passed, failed, error, running
	CallSetupTimeMs   float64        `json:"call_setup_time_ms"`
	RTPPacketsSent    uint64         `json:"rtp_packets_sent"`
	RTPPacketsReceived uint64        `json:"rtp_packets_received"`
	PacketLossPct     float64        `json:"packet_loss_pct"`
	JitterAvgMs       float64        `json:"jitter_avg_ms"`
	CallDurationS     float64        `json:"call_duration_s"`
	Codec             string         `json:"codec"`
	SIPFlow           []SIPFlowEvent `json:"sip_flow"`
	Error             string         `json:"error,omitempty"`
	StartTime         time.Time      `json:"start_time"`
	EndTime           time.Time      `json:"end_time"`
}

// SIPFlowEvent records a single SIP signaling event.
type SIPFlowEvent struct {
	Direction string `json:"direction"`
	Method    string `json:"method,omitempty"`
	Status    int    `json:"status,omitempty"`
	Timestamp string `json:"timestamp"`
}

// Suite holds a collection of test reports.
type Suite struct {
	Name      string        `json:"name"`
	Timestamp string        `json:"timestamp"`
	Reports   []*TestReport `json:"reports"`
	Summary   Summary       `json:"summary"`
}

// Summary aggregates test results.
type Summary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Errors int `json:"errors"`
}

// NewSuite creates a report suite from test reports.
func NewSuite(name string, reports []*TestReport) *Suite {
	s := &Suite{
		Name:      name,
		Timestamp: time.Now().Format(time.RFC3339),
		Reports:   reports,
	}
	s.Summary.Total = len(reports)
	for _, r := range reports {
		switch r.Status {
		case "passed":
			s.Summary.Passed++
		case "failed":
			s.Summary.Failed++
		case "error":
			s.Summary.Errors++
		}
	}
	return s
}

// WriteJSON writes the report suite to a JSON file.
func (s *Suite) WriteJSON(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := filepath.Join(dir, fmt.Sprintf("report_%s.json", time.Now().Format("20060102_150405")))
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return filename, os.WriteFile(filename, data, 0644)
}

// WriteHTML writes the report suite to an HTML file.
func (s *Suite) WriteHTML(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	filename := filepath.Join(dir, fmt.Sprintf("report_%s.html", time.Now().Format("20060102_150405")))

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return filename, tmpl.Execute(f, s)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SIP Test Report - {{.Name}}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  .container { max-width: 1200px; margin: 0 auto; }
  h1 { font-size: 1.8rem; margin-bottom: 0.5rem; color: #f8fafc; }
  .timestamp { color: #94a3b8; margin-bottom: 2rem; }
  .summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; margin-bottom: 2rem; }
  .summary-card { background: #1e293b; border-radius: 8px; padding: 1.5rem; text-align: center; }
  .summary-card .number { font-size: 2rem; font-weight: bold; }
  .summary-card .label { color: #94a3b8; font-size: 0.875rem; margin-top: 0.25rem; }
  .passed .number { color: #4ade80; }
  .failed .number { color: #f87171; }
  .errors .number { color: #fbbf24; }
  .total .number { color: #60a5fa; }
  .test-card { background: #1e293b; border-radius: 8px; padding: 1.5rem; margin-bottom: 1rem; border-left: 4px solid #475569; }
  .test-card.status-passed { border-left-color: #4ade80; }
  .test-card.status-failed { border-left-color: #f87171; }
  .test-card.status-error { border-left-color: #fbbf24; }
  .test-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
  .test-name { font-size: 1.2rem; font-weight: 600; }
  .badge { padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; }
  .badge-passed { background: #065f46; color: #4ade80; }
  .badge-failed { background: #7f1d1d; color: #f87171; }
  .badge-error { background: #78350f; color: #fbbf24; }
  .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; margin-bottom: 1rem; }
  .metric { background: #0f172a; border-radius: 6px; padding: 0.75rem; }
  .metric .value { font-size: 1.1rem; font-weight: 600; color: #f8fafc; }
  .metric .label { font-size: 0.75rem; color: #94a3b8; }
  .sip-flow { margin-top: 1rem; }
  .sip-flow h3 { font-size: 0.875rem; color: #94a3b8; margin-bottom: 0.5rem; }
  .flow-event { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; font-family: monospace; font-size: 0.8rem; }
  .flow-arrow { color: #60a5fa; }
  .flow-arrow.recv { color: #4ade80; }
  .error-msg { color: #f87171; background: #450a0a; padding: 0.75rem; border-radius: 6px; margin-top: 0.5rem; font-size: 0.875rem; }
</style>
</head>
<body>
<div class="container">
  <h1>SIP Test Report</h1>
  <p class="timestamp">{{.Name}} &mdash; {{.Timestamp}}</p>

  <div class="summary">
    <div class="summary-card total"><div class="number">{{.Summary.Total}}</div><div class="label">Total Tests</div></div>
    <div class="summary-card passed"><div class="number">{{.Summary.Passed}}</div><div class="label">Passed</div></div>
    <div class="summary-card failed"><div class="number">{{.Summary.Failed}}</div><div class="label">Failed</div></div>
    <div class="summary-card errors"><div class="number">{{.Summary.Errors}}</div><div class="label">Errors</div></div>
  </div>

  {{range .Reports}}
  <div class="test-card status-{{.Status}}">
    <div class="test-header">
      <span class="test-name">{{.TestName}}</span>
      <span class="badge badge-{{.Status}}">{{.Status}}</span>
    </div>
    <div class="metrics">
      <div class="metric"><div class="value">{{printf "%.0f" .CallSetupTimeMs}}ms</div><div class="label">Call Setup Time</div></div>
      <div class="metric"><div class="value">{{printf "%.1f" .CallDurationS}}s</div><div class="label">Call Duration</div></div>
      <div class="metric"><div class="value">{{.RTPPacketsSent}}</div><div class="label">RTP Packets Sent</div></div>
      <div class="metric"><div class="value">{{.RTPPacketsReceived}}</div><div class="label">RTP Packets Received</div></div>
      <div class="metric"><div class="value">{{printf "%.2f" .PacketLossPct}}%</div><div class="label">Packet Loss</div></div>
      <div class="metric"><div class="value">{{.Codec}}</div><div class="label">Codec</div></div>
    </div>
    {{if .Error}}<div class="error-msg">{{.Error}}</div>{{end}}
    {{if .SIPFlow}}
    <div class="sip-flow">
      <h3>SIP Flow</h3>
      {{range .SIPFlow}}
      <div class="flow-event">
        {{if eq .Direction "sent"}}<span class="flow-arrow">&#8594;</span> SENT{{else}}<span class="flow-arrow recv">&#8592;</span> RECV{{end}}
        {{if .Method}}<strong>{{.Method}}</strong>{{end}}
        {{if .Status}}<strong>{{.Status}}</strong>{{end}}
        <span style="color:#64748b">{{.Timestamp}}</span>
      </div>
      {{end}}
    </div>
    {{end}}
  </div>
  {{end}}
</div>
</body>
</html>`
