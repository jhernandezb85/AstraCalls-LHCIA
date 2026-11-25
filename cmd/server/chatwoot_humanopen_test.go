package main

import "testing"

func TestHumanReplyOpens(t *testing.T) {
	mk := func(status, senderType string) map[string]any {
		return map[string]any{
			"conversation": map[string]any{"id": 1.0, "status": status},
			"sender":       map[string]any{"type": senderType},
		}
	}
	cases := []struct {
		status, sender string
		want           bool
	}{
		{"pending", "user", true},
		{"pending", "captain_assistant", false},
		{"pending", "agent_bot", false},
		{"pending", "", false},
		{"open", "user", false},
		{"resolved", "user", false},
	}
	for _, c := range cases {
		if got := humanReplyOpens(mk(c.status, c.sender)); got != c.want {
			t.Errorf("status=%s sender=%s: got %v want %v", c.status, c.sender, got, c.want)
		}
	}
}
