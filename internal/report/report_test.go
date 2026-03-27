package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSuiteSummary(t *testing.T) {
	reports := []*TestReport{
		{TestName: "test1", Status: "passed"},
		{TestName: "test2", Status: "passed"},
		{TestName: "test3", Status: "failed"},
		{TestName: "test4", Status: "error"},
		{TestName: "test5", Status: "failed"},
	}

	suite := NewSuite("test-run", reports)

	if suite.Name != "test-run" {
		t.Errorf("Name: got %q, want %q", suite.Name, "test-run")
	}
	if suite.Summary.Total != 5 {
		t.Errorf("Total: got %d, want 5", suite.Summary.Total)
	}
	if suite.Summary.Passed != 2 {
		t.Errorf("Passed: got %d, want 2", suite.Summary.Passed)
	}
	if suite.Summary.Failed != 2 {
		t.Errorf("Failed: got %d, want 2", suite.Summary.Failed)
	}
	if suite.Summary.Errors != 1 {
		t.Errorf("Errors: got %d, want 1", suite.Summary.Errors)
	}
	if len(suite.Reports) != 5 {
		t.Errorf("Reports: got %d, want 5", len(suite.Reports))
	}
}

func TestNewSuiteEmpty(t *testing.T) {
	suite := NewSuite("empty", nil)
	if suite.Summary.Total != 0 {
		t.Errorf("Total: got %d, want 0", suite.Summary.Total)
	}
	if suite.Summary.Passed != 0 || suite.Summary.Failed != 0 || suite.Summary.Errors != 0 {
		t.Error("empty suite should have all zero counts")
	}
}

func TestNewSuiteTimestamp(t *testing.T) {
	suite := NewSuite("ts-test", nil)
	_, err := time.Parse(time.RFC3339, suite.Timestamp)
	if err != nil {
		t.Errorf("Timestamp not valid RFC3339: %q, err: %v", suite.Timestamp, err)
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	reports := []*TestReport{
		{
			TestName:         "json-test",
			Status:           "passed",
			CallSetupTimeMs:  150.5,
			RTPPacketsSent:   1500,
			RTPPacketsReceived: 1480,
			PacketLossPct:    1.33,
			CallDurationS:    30.0,
			Codec:            "PCMU",
			SIPFlow: []SIPFlowEvent{
				{Direction: "sent", Method: "INVITE", Timestamp: "2024-01-01T00:00:00Z"},
				{Direction: "recv", Status: 200, Timestamp: "2024-01-01T00:00:01Z"},
			},
		},
	}

	suite := NewSuite("json-suite", reports)
	filename, err := suite.WriteJSON(dir)
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	if !strings.HasPrefix(filepath.Base(filename), "report_") {
		t.Errorf("filename should start with report_: got %q", filepath.Base(filename))
	}
	if !strings.HasSuffix(filename, ".json") {
		t.Errorf("filename should end with .json: got %q", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var parsed Suite
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Name != "json-suite" {
		t.Errorf("Name: got %q, want %q", parsed.Name, "json-suite")
	}
	if len(parsed.Reports) != 1 {
		t.Fatalf("Reports: got %d, want 1", len(parsed.Reports))
	}
	if parsed.Reports[0].TestName != "json-test" {
		t.Errorf("TestName: got %q, want %q", parsed.Reports[0].TestName, "json-test")
	}
	if parsed.Reports[0].RTPPacketsSent != 1500 {
		t.Errorf("RTPPacketsSent: got %d, want 1500", parsed.Reports[0].RTPPacketsSent)
	}
}

func TestWriteJSONCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	suite := NewSuite("nested", nil)
	_, err := suite.WriteJSON(dir)
	if err != nil {
		t.Fatalf("WriteJSON should create nested dirs: %v", err)
	}
}

func TestWriteHTML(t *testing.T) {
	dir := t.TempDir()
	reports := []*TestReport{
		{
			TestName:        "html-test",
			Status:          "passed",
			CallSetupTimeMs: 200,
			CallDurationS:   10.0,
			Codec:           "PCMA",
		},
		{
			TestName: "html-fail",
			Status:   "failed",
			Error:    "connection refused",
		},
	}

	suite := NewSuite("html-suite", reports)
	filename, err := suite.WriteHTML(dir)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	if !strings.HasSuffix(filename, ".html") {
		t.Errorf("filename should end with .html: got %q", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	content := string(data)
	checks := []string{
		"<!DOCTYPE html>",
		"html-suite",
		"html-test",
		"html-fail",
		"connection refused",
		"PCMA",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("HTML missing %q", c)
		}
	}
}

func TestWriteHTMLCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	suite := NewSuite("nested", nil)
	_, err := suite.WriteHTML(dir)
	if err != nil {
		t.Fatalf("WriteHTML should create nested dirs: %v", err)
	}
}
