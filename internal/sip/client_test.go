package sip

import "testing"

func TestCallStateString(t *testing.T) {
	tests := []struct {
		state CallState
		want  string
	}{
		{StateIdle, "idle"},
		{StateInviteSent, "invite_sent"},
		{StateTrying, "trying"},
		{StateRinging, "ringing"},
		{StateAnswered, "answered"},
		{StateActive, "active"},
		{StateBye, "bye"},
		{StateFailed, "failed"},
		{StateTimeout, "timeout"},
		{CallState(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("CallState(%d).String(): got %q, want %q", tt.state, got, tt.want)
		}
	}
}
